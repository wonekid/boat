package controller

import (
	"boat/internal/database"
	"boat/internal/model"
	"boat/internal/utils"

	"github.com/gin-gonic/gin"
)

// ListHosts 主机列表
func ListHosts(c *gin.Context) {
	page, pageSize := utils.ParsePage(c.Query("page"), c.Query("pageSize"))
	keyword := c.Query("keyword")
	groupID := c.Query("groupId")

	q := database.DB.Model(&model.Host{})
	if keyword != "" {
		q = q.Where("name LIKE ? OR ip LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if groupID != "" {
		q = q.Where("group_id = ?", groupID)
	}
	var total int64
	q.Count(&total)
	var list []model.Host
	q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list)
	utils.Page(c, list, total, page, pageSize)
}

// HostReq 主机请求
type HostReq struct {
	Name         string `json:"name" binding:"required"`
	IP           string `json:"ip" binding:"required"`
	Port         int    `json:"port"`
	OS           string `json:"os"`
	Status       int    `json:"status"`
	GroupID      uint   `json:"groupId"`
	CredentialID uint   `json:"credentialId"`
	User         string `json:"user"`
	CPU          string `json:"cpu"`
	Mem          string `json:"mem"`
	Disk         string `json:"disk"`
	Remark       string `json:"remark"`
}

// CreateHost 新建主机
func CreateHost(c *gin.Context) {
	var req HostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	host := model.Host{
		Name: req.Name, IP: req.IP, Port: req.Port, OS: req.OS,
		Status: req.Status, GroupID: req.GroupID, CredentialID: req.CredentialID,
		User: req.User, CPU: req.CPU, Mem: req.Mem, Disk: req.Disk, Remark: req.Remark,
	}
	if host.Port == 0 {
		host.Port = 22
	}
	database.DB.Create(&host)
	utils.SuccessMsg(c, "创建成功")
}

// UpdateHost 更新主机
func UpdateHost(c *gin.Context) {
	id := c.Param("id")
	var req HostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Fail(c, "参数错误")
		return
	}
	var host model.Host
	if err := database.DB.First(&host, id).Error; err != nil {
		utils.Fail(c, "主机不存在")
		return
	}
	host.Name = req.Name
	host.IP = req.IP
	host.Port = req.Port
	host.OS = req.OS
	host.Status = req.Status
	host.GroupID = req.GroupID
	host.CredentialID = req.CredentialID
	host.User = req.User
	host.CPU = req.CPU
	host.Mem = req.Mem
	host.Disk = req.Disk
	host.Remark = req.Remark
	database.DB.Save(&host)
	utils.SuccessMsg(c, "更新成功")
}

// DeleteHost 删除主机
func DeleteHost(c *gin.Context) {
	id := c.Param("id")
	database.DB.Delete(&model.Host{}, id)
	utils.SuccessMsg(c, "删除成功")
}

// GetHost 主机详情
func GetHost(c *gin.Context) {
	id := c.Param("id")
	var host model.Host
	if err := database.DB.First(&host, id).Error; err != nil {
		utils.Fail(c, "主机不存在")
		return
	}
	utils.Success(c, host)
}

// BatchCreateHosts 批量导入主机
func BatchCreateHosts(c *gin.Context) {
	var req struct {
		Hosts []HostReq `json:"hosts"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Hosts) == 0 {
		utils.Fail(c, "参数错误")
		return
	}
	var hosts []model.Host
	for _, h := range req.Hosts {
		port := h.Port
		if port == 0 {
			port = 22
		}
		hosts = append(hosts, model.Host{
			Name: h.Name, IP: h.IP, Port: port, OS: h.OS, Status: h.Status,
			GroupID: h.GroupID, CredentialID: h.CredentialID, User: h.User,
			Remark: h.Remark,
		})
	}
	database.DB.CreateInBatches(hosts, 100)
	utils.Success(c, gin.H{"count": len(hosts)})
}
