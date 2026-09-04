package osp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"
)

// Options 服务端参数
type Options struct {
	// Port 自定义监听端口（执行机 Agent 主动外连至此端口）
	Port int
	// Heartbeat 下发给 Agent 的心跳周期（秒）
	Heartbeat int
	// OfflineAfter 超过该秒数未收到心跳即判定离线
	OfflineAfter int
	// HandshakeTimeout 握手超时（秒）
	HandshakeTimeout int
	// DBThrottle 心跳写库节流间隔（秒），避免高频写库
	DBThrottle int
	// Enabled 是否启用 OSP 服务
	Enabled bool
}

// DefaultOptions 默认参数
func DefaultOptions() Options {
	return Options{
		Port:             9090,
		Heartbeat:        10,
		OfflineAfter:     35,
		HandshakeTimeout: 20,
		DBThrottle:       5,
		Enabled:          true,
	}
}

// EventType 状态事件类型（供前端 WS 实时刷新）
const (
	EventOnline     = "node_online"
	EventOffline    = "node_offline"
	EventMetrics    = "node_metrics"
	EventTaskResult = "task_result"
)

// Event 推送给控制台订阅者的实时事件
type Event struct {
	Type      string             `json:"type"`
	Node      *NodeStatus        `json:"node,omitempty"`
	Result    *TaskResultPayload `json:"result,omitempty"`
	Timestamp int64              `json:"timestamp"`
}

// NodeStatus 节点实时状态快照（WS 推送与 API 返回共用）
type NodeStatus struct {
	ID         uint       `json:"id"`
	Name       string     `json:"name"`
	Hostname   string     `json:"hostname"`
	IP         string     `json:"ip"`
	OS         string     `json:"os"`
	Arch       string     `json:"arch"`
	Status     string     `json:"status"` // online | offline
	Enabled    int        `json:"enabled"`
	Labels     string     `json:"labels"`
	Version    string     `json:"version"`
	CPUUsage   float64    `json:"cpuUsage"`
	MemUsage   float64    `json:"memUsage"`
	DiskUsage  float64    `json:"diskUsage"`
	LoadAvg    string     `json:"loadAvg"`
	Uptime     int64      `json:"uptime"`
	MemTotal   uint64     `json:"memTotal"`
	MemUsed    uint64     `json:"memUsed"`
	DiskTotal  uint64     `json:"diskTotal"`
	DiskUsed   uint64     `json:"diskUsed"`
	LastSeenAt *time.Time `json:"lastSeenAt"`
}

// NodeStatusFromModel 由数据库模型构造状态快照
func NodeStatusFromModel(n model.AgentNode) *NodeStatus {
	return &NodeStatus{
		ID: n.ID, Name: n.Name, Hostname: n.Hostname, IP: n.IP, OS: n.OS, Arch: n.Arch,
		Status: n.Status, Enabled: n.Enabled, Labels: n.Labels, Version: n.Version,
		CPUUsage: n.CPUUsage, MemUsage: n.MemUsage, DiskUsage: n.DiskUsage, LoadAvg: n.LoadAvg,
		Uptime: n.Uptime, MemTotal: n.MemTotal, MemUsed: n.MemUsed,
		DiskTotal: n.DiskTotal, DiskUsed: n.DiskUsed, LastSeenAt: n.LastSeenAt,
	}
}

// ---------- 连接 ----------

// Conn 一条 Agent 长连接
type Conn struct {
	net.Conn
	sess *Session

	nodeID   uint
	nodeName string
	remoteIP string

	mu       sync.Mutex
	lastSeen time.Time
	metrics  Metrics
	lastDB   time.Time
	online   bool

	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[uint]chan *TaskResultPayload

	closeOnce sync.Once
	closed    chan struct{}
}

func newConn(raw net.Conn) *Conn {
	host := ""
	if tcpAddr, ok := raw.RemoteAddr().(*net.TCPAddr); ok {
		host = tcpAddr.IP.String()
	}
	return &Conn{
		Conn:     raw,
		remoteIP: host,
		lastSeen: time.Now(),
		pending:  make(map[uint]chan *TaskResultPayload),
		closed:   make(chan struct{}),
	}
}

// Send 发送一条加密消息
func (c *Conn) Send(msgType string, payload interface{}) error {
	env, err := NewEnvelope(msgType, payload)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return err
	}
	return WriteFrame(c.Conn, c.sess, env)
}

// Close 关闭连接（幂等）
func (c *Conn) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		_ = c.Conn.Close()
	})
}

// Done 连接关闭信号
func (c *Conn) Done() <-chan struct{} { return c.closed }

// Dispatch 下发任务并同步等待该节点回传结果
func (c *Conn) Dispatch(ctx context.Context, payload *TaskPayload) (*TaskResultPayload, error) {
	ch := make(chan *TaskResultPayload, 1)
	c.pendingMu.Lock()
	c.pending[payload.ResultID] = ch
	c.pendingMu.Unlock()
	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, payload.ResultID)
		c.pendingMu.Unlock()
	}()

	if err := c.Send(MsgTask, payload); err != nil {
		return nil, err
	}
	wait := time.Duration(payload.Timeout)*time.Second + 30*time.Second
	if wait < 60*time.Second {
		wait = 60 * time.Second
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case res := <-ch:
		return res, nil
	case <-timer.C:
		return nil, ErrTimeout
	case <-c.closed:
		return nil, ErrDisconnected
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// deliver 将收到的结果投递给等待中的下发请求
func (c *Conn) deliver(res *TaskResultPayload) bool {
	c.pendingMu.Lock()
	ch, ok := c.pending[res.ResultID]
	c.pendingMu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- res:
	default:
	}
	return true
}

// Cancel 通知 Agent 取消任务
func (c *Conn) Cancel(resultID uint) error {
	return c.Send(MsgCancel, &CancelPayload{ResultID: resultID})
}

// Status 当前状态快照
func (c *Conn) Status() *NodeStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	st := c.metrics
	return &NodeStatus{
		ID: c.nodeID, Name: c.nodeName, Status: StatusOnline,
		CPUUsage: st.CPUUsage, MemUsage: st.MemUsage, DiskUsage: st.DiskUsage,
		LoadAvg: st.LoadAvg, Uptime: st.Uptime, MemTotal: st.MemTotal, MemUsed: st.MemUsed,
		DiskTotal: st.DiskTotal, DiskUsed: st.DiskUsed, Version: st.AgentVer,
		LastSeenAt: func() *time.Time { t := c.lastSeen; return &t }(),
	}
}

// ---------- Hub ----------

// Subscriber 事件订阅者（控制台 WS）
type Subscriber struct {
	ch chan Event
}

// Hub 连接注册表与事件总线
type Hub struct {
	mu    sync.RWMutex
	conns map[uint]*Conn
	subs  map[*Subscriber]struct{}
}

func newHub() *Hub {
	return &Hub{conns: make(map[uint]*Conn), subs: make(map[*Subscriber]struct{})}
}

// Put 注册连接；若该节点已有旧连接则先踢掉（防止重复登录）
func (h *Hub) Put(nodeID uint, c *Conn) {
	h.mu.Lock()
	if old, ok := h.conns[nodeID]; ok && old != c {
		h.mu.Unlock()
		old.Close()
		h.mu.Lock()
	}
	h.conns[nodeID] = c
	h.mu.Unlock()
}

// Remove 移除连接（仅当传入连接仍是当前连接时）
func (h *Hub) Remove(nodeID uint, c *Conn) {
	h.mu.Lock()
	if cur, ok := h.conns[nodeID]; ok && cur == c {
		delete(h.conns, nodeID)
	}
	h.mu.Unlock()
}

// Get 取节点当前连接
func (h *Hub) Get(nodeID uint) *Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.conns[nodeID]
}

// OnlineCount 在线连接数
func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// OnlineIDs 在线节点 ID 列表
func (h *Hub) OnlineIDs() []uint {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]uint, 0, len(h.conns))
	for id := range h.conns {
		ids = append(ids, id)
	}
	return ids
}

// Subscribe 订阅实时事件
func (h *Hub) Subscribe() *Subscriber {
	s := &Subscriber{ch: make(chan Event, 64)}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

// Unsubscribe 取消订阅
func (h *Hub) Unsubscribe(s *Subscriber) {
	h.mu.Lock()
	if _, ok := h.subs[s]; ok {
		delete(h.subs, s)
		close(s.ch)
	}
	h.mu.Unlock()
}

// Publish 广播事件（订阅者通道满时丢弃，避免阻塞业务）
func (h *Hub) Publish(e Event) {
	e.Timestamp = time.Now().UnixMilli()
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		select {
		case s.ch <- e:
		default:
		}
	}
}

// Events 订阅者事件通道
func (s *Subscriber) Events() <-chan Event { return s.ch }

// ---------- Server ----------

// Server OSP 服务端
type Server struct {
	opt   Options
	hub   *Hub
	ln    net.Listener
	quit  chan struct{}
	stop  sync.Once
	ready chan struct{}
}

var (
	defaultServer *Server
	serverMu      sync.RWMutex
)

// Init 初始化默认服务端实例
func Init(opt Options) {
	serverMu.Lock()
	defer serverMu.Unlock()
	defaultServer = &Server{
		opt:   opt,
		hub:   newHub(),
		quit:  make(chan struct{}),
		ready: make(chan struct{}),
	}
}

// Default 取默认服务端实例（未初始化则 nil）
func Default() *Server {
	serverMu.RLock()
	defer serverMu.RUnlock()
	return defaultServer
}

// Hub 取连接注册表
func (s *Server) Hub() *Hub { return s.hub }

// Options 取运行参数
func (s *Server) Options() Options { return s.opt }

// Start 启动监听（阻塞）
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.opt.Port))
	if err != nil {
		return fmt.Errorf("OSP 服务监听 :%d 失败: %w", s.opt.Port, err)
	}
	s.ln = ln
	close(s.ready)
	log.Printf("[osp] OSP Agent 服务已启动，监听端口 %d（自定义加密协议）", s.opt.Port)

	go s.offlineLoop()

	for {
		raw, err := ln.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return nil
			default:
				log.Printf("[osp] 接受连接失败: %v", err)
				continue
			}
		}
		go s.handleConn(raw)
	}
}

// Shutdown 停止服务
func (s *Server) Shutdown() {
	s.stop.Do(func() {
		close(s.quit)
		if s.ln != nil {
			_ = s.ln.Close()
		}
	})
}

// WaitReady 等待监听就绪（测试/编排用）
func (s *Server) WaitReady() {
	if s.ready == nil {
		return
	}
	<-s.ready
}

// Dispatch 向指定节点下发任务（节点离线返回错误）
func (s *Server) Dispatch(ctx context.Context, nodeID uint, payload *TaskPayload) (*TaskResultPayload, error) {
	c := s.hub.Get(nodeID)
	if c == nil {
		return nil, ErrDisconnected
	}
	return c.Dispatch(ctx, payload)
}

// CancelTask 取消指定节点上的任务
func (s *Server) CancelTask(nodeID, resultID uint) error {
	c := s.hub.Get(nodeID)
	if c == nil {
		return ErrDisconnected
	}
	return c.Cancel(resultID)
}

// Kick 强制断开节点连接
func (s *Server) Kick(nodeID uint) bool {
	c := s.hub.Get(nodeID)
	if c == nil {
		return false
	}
	c.Close()
	return true
}

// handleConn 处理单条 Agent 连接：握手 → 注册 → 读循环
func (s *Server) handleConn(raw net.Conn) {
	c := newConn(raw)
	defer func() {
		if c.nodeID > 0 {
			s.hub.Remove(c.nodeID, c)
			s.markNodeOffline(c, false)
		}
		c.Close()
	}()

	if err := raw.SetDeadline(time.Now().Add(time.Duration(s.opt.HandshakeTimeout) * time.Second)); err != nil {
		return
	}
	if err := s.handshake(c); err != nil {
		log.Printf("[osp] 握手失败(%s): %v", c.remoteIP, err)
		// 尽量告知对端原因
		_ = writePlain(raw, MsgReject, &RejectPayload{Reason: err.Error()})
		return
	}
	// 握手后交由读循环管理超时
	_ = raw.SetDeadline(time.Time{})
	s.readLoop(c)
}

// writePlain 写明文帧（仅握手阶段）
func writePlain(w net.Conn, msgType string, payload interface{}) error {
	env, err := NewEnvelope(msgType, payload)
	if err != nil {
		return err
	}
	return WriteFrame(w, nil, env)
}

// handshake 完成身份认证与密钥协商
func (s *Server) handshake(c *Conn) error {
	env, err := ReadFrame(c.Conn, nil)
	if err != nil {
		return fmt.Errorf("读取握手帧失败: %w", err)
	}
	if env.Type != MsgHello {
		return fmt.Errorf("期望 hello 消息，实际收到 %s", env.Type)
	}
	var hello HelloPayload
	if err := env.Decode(&hello); err != nil {
		return fmt.Errorf("握手数据解析失败: %w", err)
	}
	if hello.Token == "" {
		return ErrBadToken
	}

	// 令牌校验（接入即认证）
	var node model.AgentNode
	if err := database.DB.Where("token = ?", hello.Token).First(&node).Error; err != nil {
		return ErrBadToken
	}
	if node.Enabled != 1 {
		return ErrNodeDisabled
	}

	// ECDH 密钥协商
	kex, err := NewKeyExchange()
	if err != nil {
		return err
	}
	nonceA, err := base64.StdEncoding.DecodeString(hello.NonceA)
	if err != nil || len(nonceA) != 16 {
		return fmt.Errorf("nonceA 非法")
	}
	pubA, err := base64.StdEncoding.DecodeString(hello.ECDHPubA)
	if err != nil {
		return fmt.Errorf("ECDH 公钥非法")
	}
	if err := ecdhPubCheck(pubA); err != nil {
		return err
	}
	nonceBRaw, nonceB, err := RandomNonce()
	if err != nil {
		return err
	}

	// 服务端对 (nonceA || nonceB || ecdhPubB) 签名，Agent 用预置公钥验签，防中间人
	signed := make([]byte, 0, len(nonceA)+len(nonceBRaw)+len(kex.Pub))
	signed = append(signed, nonceA...)
	signed = append(signed, nonceBRaw...)
	signed = append(signed, kex.Pub...)
	signature, err := utils.SignData(signed)
	if err != nil {
		return fmt.Errorf("服务端签名失败: %w", err)
	}

	welcome := &WelcomePayload{
		SessionID:  newSessionID(),
		NodeID:     node.ID,
		NodeName:   node.Name,
		NonceB:     nonceB,
		ECDHPubB:   base64.StdEncoding.EncodeToString(kex.Pub),
		Signature:  signature,
		Heartbeat:  s.opt.Heartbeat,
		ServerTime: time.Now().Unix(),
	}
	if err := writePlain(c.Conn, MsgWelcome, welcome); err != nil {
		return err
	}

	sess, err := kex.Derive(pubA, nonceA, nonceBRaw, hello.Token)
	if err != nil {
		return err
	}

	c.sess = sess
	c.nodeID = node.ID
	c.nodeName = node.Name
	c.mu.Lock()
	c.metrics = Metrics{AgentVer: hello.Version}
	c.mu.Unlock()

	// 更新节点档案（注册信息以 Agent 上报为准，IP 为空时回退 TCP 远端地址）
	ip := hello.IP
	if ip == "" {
		ip = c.remoteIP
	}
	now := time.Now()
	updates := map[string]interface{}{
		"status":       StatusOnline,
		"last_seen_at": now,
		"hostname":     hello.Hostname,
		"os":           hello.OS,
		"arch":         hello.Arch,
		"ip":           ip,
		"version":      hello.Version,
	}
	if hello.Labels != "" {
		updates["labels"] = hello.Labels
	}
	if node.RegisteredAt == nil {
		updates["registered_at"] = now
	}
	database.DB.Model(&model.AgentNode{}).Where("id = ?", node.ID).Updates(updates)

	c.mu.Lock()
	c.online = true
	c.lastSeen = now
	c.lastDB = now
	c.mu.Unlock()

	s.hub.Put(node.ID, c)
	log.Printf("[osp] 节点上线: %s(id=%d) %s 来自 %s", node.Name, node.ID, ip, c.remoteIP)
	s.publishNodeStatus(EventOnline, node.ID)
	return nil
}

// readLoop 读循环：心跳、任务结果
func (s *Server) readLoop(c *Conn) {
	idle := time.Duration(s.opt.Heartbeat*3+30) * time.Second
	for {
		if err := c.Conn.SetReadDeadline(time.Now().Add(idle)); err != nil {
			return
		}
		env, err := ReadFrame(c.Conn, c.sess)
		if err != nil {
			log.Printf("[osp] 节点 %s(id=%d) 读取失败: %v", c.nodeName, c.nodeID, err)
			return
		}
		switch env.Type {
		case MsgHeartbeat:
			var hb HeartbeatPayload
			if err := env.Decode(&hb); err != nil {
				continue
			}
			s.onHeartbeat(c, &hb)
		case MsgTaskResult:
			var res TaskResultPayload
			if err := env.Decode(&res); err != nil {
				continue
			}
			c.deliver(&res)
			s.hub.Publish(Event{Type: EventTaskResult, Result: &res})
		case MsgPong:
			// 探活响应，忽略
		default:
			log.Printf("[osp] 未知消息类型: %s", env.Type)
		}
	}
}

// onHeartbeat 处理心跳：更新内存状态、节流写库、广播实时指标
func (s *Server) onHeartbeat(c *Conn, hb *HeartbeatPayload) {
	now := time.Now()
	m := hb.Metrics
	c.mu.Lock()
	first := !c.online
	c.online = true
	c.metrics = m
	c.lastSeen = now
	needDB := first || now.Sub(c.lastDB) >= time.Duration(s.opt.DBThrottle)*time.Second
	if needDB {
		c.lastDB = now
	}
	c.mu.Unlock()

	if needDB {
		database.DB.Model(&model.AgentNode{}).Where("id = ?", c.nodeID).Updates(map[string]interface{}{
			"status":       StatusOnline,
			"last_seen_at": now,
			"cpu_usage":    m.CPUUsage,
			"mem_usage":    m.MemUsage,
			"disk_usage":   m.DiskUsage,
			"load_avg":     m.LoadAvg,
			"uptime":       m.Uptime,
			"mem_total":    m.MemTotal,
			"mem_used":     m.MemUsed,
			"disk_total":   m.DiskTotal,
			"disk_used":    m.DiskUsed,
			"version":      m.AgentVer,
		})
	}
	if first {
		s.publishNodeStatus(EventOnline, c.nodeID)
		return
	}
	s.hub.Publish(Event{Type: EventMetrics, Node: s.nodeStatus(c.nodeID, m, now)})
}

// publishNodeStatus 从数据库读取最新节点信息并广播
func (s *Server) publishNodeStatus(eventType string, nodeID uint) {
	var node model.AgentNode
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		return
	}
	if eventType == EventOffline {
		node.Status = StatusOffline
	}
	st := NodeStatusFromModel(node)
	if c := s.hub.Get(nodeID); c != nil {
		live := c.Status()
		st.CPUUsage, st.MemUsage, st.DiskUsage = live.CPUUsage, live.MemUsage, live.DiskUsage
		st.LoadAvg, st.Uptime = live.LoadAvg, live.Uptime
		st.LastSeenAt = live.LastSeenAt
	}
	s.hub.Publish(Event{Type: eventType, Node: st})
}

func (s *Server) nodeStatus(nodeID uint, m Metrics, now time.Time) *NodeStatus {
	var node model.AgentNode
	if err := database.DB.First(&node, nodeID).Error; err == nil {
		st := NodeStatusFromModel(node)
		st.Status = StatusOnline
		st.CPUUsage, st.MemUsage, st.DiskUsage = m.CPUUsage, m.MemUsage, m.DiskUsage
		st.LoadAvg, st.Uptime = m.LoadAvg, m.Uptime
		st.MemTotal, st.MemUsed = m.MemTotal, m.MemUsed
		st.DiskTotal, st.DiskUsed = m.DiskTotal, m.DiskUsed
		st.Version = m.AgentVer
		st.LastSeenAt = &now
		return st
	}
	lastSeen := now
	return &NodeStatus{ID: nodeID, Name: s.nameOf(nodeID), Status: StatusOnline, LastSeenAt: &lastSeen}
}

func (s *Server) nameOf(nodeID uint) string {
	var node model.AgentNode
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		return ""
	}
	return node.Name
}

// markNodeOffline 将节点置为离线（force=true 时强制写库）
func (s *Server) markNodeOffline(c *Conn, force bool) {
	if c.nodeID == 0 {
		return
	}
	c.mu.Lock()
	if !c.online && !force {
		c.mu.Unlock()
		return
	}
	c.online = false
	c.mu.Unlock()

	// 若 Hub 中仍是另一条连接（agent 重连），不覆盖在线状态
	if cur := s.hub.Get(c.nodeID); cur != nil && cur != c {
		return
	}
	database.DB.Model(&model.AgentNode{}).Where("id = ? AND status = ?", c.nodeID, StatusOnline).
		Update("status", StatusOffline)
	log.Printf("[osp] 节点离线: %s(id=%d)", c.nodeName, c.nodeID)
	s.publishNodeStatus(EventOffline, c.nodeID)
}

// offlineLoop 定期扫描超时未心跳的节点并置离线
func (s *Server) offlineLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.quit:
			return
		case <-ticker.C:
			s.sweepOffline()
		}
	}
}

// sweepOffline 扫描并清理超时节点
func (s *Server) sweepOffline() {
	threshold := time.Now().Add(-time.Duration(s.opt.OfflineAfter) * time.Second)
	var nodes []model.AgentNode
	if err := database.DB.Where("status = ? AND (last_seen_at IS NULL OR last_seen_at < ?)", StatusOnline, threshold).
		Find(&nodes).Error; err != nil {
		return
	}
	for _, n := range nodes {
		c := s.hub.Get(n.ID)
		if c != nil {
			// 连接仍在但心跳超时，断开后由连接清理逻辑置离线
			c.Close()
			continue
		}
		database.DB.Model(&model.AgentNode{}).Where("id = ?", n.ID).Update("status", StatusOffline)
		s.publishNodeStatus(EventOffline, n.ID)
	}
}

// ---------- 包级便捷入口（控制台调用） ----------

// DispatchTask 向节点下发任务（使用默认服务端实例）
func DispatchTask(ctx context.Context, nodeID uint, payload *TaskPayload) (*TaskResultPayload, error) {
	s := Default()
	if s == nil {
		return nil, fmt.Errorf("OSP 服务未启动")
	}
	return s.Dispatch(ctx, nodeID, payload)
}

// CancelNodeTask 取消节点任务
func CancelNodeTask(nodeID, resultID uint) error {
	s := Default()
	if s == nil {
		return fmt.Errorf("OSP 服务未启动")
	}
	return s.CancelTask(nodeID, resultID)
}

// Snapshot 全量节点状态（WS 首帧推送）
func Snapshot() []*NodeStatus {
	var nodes []model.AgentNode
	if err := database.DB.Order("id ASC").Find(&nodes).Error; err != nil {
		return nil
	}
	list := make([]*NodeStatus, 0, len(nodes))
	for _, n := range nodes {
		st := NodeStatusFromModel(n)
		if st.Status == "" {
			st.Status = StatusOffline
		}
		list = append(list, st)
	}
	return list
}

// OnlineStats 在线/离线统计
func OnlineStats() (online, offline int64) {
	database.DB.Model(&model.AgentNode{}).Where("status = ?", StatusOnline).Count(&online)
	database.DB.Model(&model.AgentNode{}).Where("status <> ? OR status IS NULL", StatusOnline).Count(&offline)
	return
}

func newSessionID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000")))
	}
	return hex.EncodeToString(buf)
}

// ecdhPubCheck 校验对端 ECDH 公钥长度（P-256 未压缩点为 65 字节）
func ecdhPubCheck(pub []byte) error {
	if len(pub) != 65 {
		return fmt.Errorf("ECDH 公钥长度非法(%d)", len(pub))
	}
	return nil
}
