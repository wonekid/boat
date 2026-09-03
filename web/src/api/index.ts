import request from '@/utils/request'

// 统一返回 data
const api = {
  // 认证
  login: (data: any) => request.post('/auth/login', data).then((r) => r.data),
  mfaVerify: (data: any) => request.post('/auth/mfa/verify', data).then((r) => r.data),
  profile: () => request.get('/profile').then((r) => r.data),
  changePwd: (data: any) => request.post('/password', data).then((r) => r.data),
  captcha: () => request.get('/captcha').then((r) => r.data),
  mfaSetup: () => request.post('/mfa/setup').then((r) => r.data),
  mfaEnable: (data: any) => request.post('/mfa/enable', data).then((r) => r.data),
  mfaDisable: (data: any) => request.post('/mfa/disable', data).then((r) => r.data),

  // 用户
  listUsers: (p: any) => request.get('/users', { params: p }).then((r) => r.data),
  getUser: (id: any) => request.get(`/users/${id}`).then((r) => r.data),
  createUser: (d: any) => request.post('/users', d).then((r) => r.data),
  updateUser: (id: any, d: any) => request.put(`/users/${id}`, d).then((r) => r.data),
  deleteUser: (id: any) => request.delete(`/users/${id}`).then((r) => r.data),

  // 角色
  listRoles: () => request.get('/roles').then((r) => r.data),
  createRole: (d: any) => request.post('/roles', d).then((r) => r.data),
  updateRole: (id: any, d: any) => request.put(`/roles/${id}`, d).then((r) => r.data),
  deleteRole: (id: any) => request.delete(`/roles/${id}`).then((r) => r.data),

  // 菜单
  listMenus: () => request.get('/menus').then((r) => r.data),
  createMenu: (d: any) => request.post('/menus', d).then((r) => r.data),
  updateMenu: (id: any, d: any) => request.put(`/menus/${id}`, d).then((r) => r.data),
  deleteMenu: (id: any) => request.delete(`/menus/${id}`).then((r) => r.data),

  // 部门
  listDepts: () => request.get('/depts').then((r) => r.data),
  createDept: (d: any) => request.post('/depts', d).then((r) => r.data),
  updateDept: (id: any, d: any) => request.put(`/depts/${id}`, d).then((r) => r.data),
  deleteDept: (id: any) => request.delete(`/depts/${id}`).then((r) => r.data),

  // 主机
  listHosts: (p: any) => request.get('/hosts', { params: p }).then((r) => r.data),
  getHost: (id: any) => request.get(`/hosts/${id}`).then((r) => r.data),
  createHost: (d: any) => request.post('/hosts', d).then((r) => r.data),
  updateHost: (id: any, d: any) => request.put(`/hosts/${id}`, d).then((r) => r.data),
  deleteHost: (id: any) => request.delete(`/hosts/${id}`).then((r) => r.data),
  batchHosts: (d: any) => request.post('/hosts/batch', d).then((r) => r.data),

  // 凭证
  listCredentials: () => request.get('/credentials').then((r) => r.data),
  createCredential: (d: any) => request.post('/credentials', d).then((r) => r.data),
  updateCredential: (id: any, d: any) => request.put(`/credentials/${id}`, d).then((r) => r.data),
  deleteCredential: (id: any) => request.delete(`/credentials/${id}`).then((r) => r.data),
  testCredential: (id: any) => request.post(`/credentials/${id}/test`).then((r) => r.data),

  // 分组
  listHostGroups: () => request.get('/host-groups').then((r) => r.data),
  createHostGroup: (d: any) => request.post('/host-groups', d).then((r) => r.data),
  updateHostGroup: (id: any, d: any) => request.put(`/host-groups/${id}`, d).then((r) => r.data),
  deleteHostGroup: (id: any) => request.delete(`/host-groups/${id}`).then((r) => r.data),

  // 授权
  listAuths: (p: any) => request.get('/auths', { params: p }).then((r) => r.data),
  createAuth: (d: any) => request.post('/auths', d).then((r) => r.data),
  deleteAuth: (id: any) => request.delete(`/auths/${id}`).then((r) => r.data),

  // 会话/日志
  listSessions: (p: any) => request.get('/sessions', { params: p }).then((r) => r.data),
  terminateSession: (id: any) => request.post(`/sessions/${id}/terminate`).then((r) => r.data),
  getSessionRecording: (id: any) =>
    request.get(`/sessions/${id}/recording`, { responseType: 'text', transformResponse: [(d: any) => d] }).then((r) => r.data),
  listAudits: (p: any) => request.get('/audits', { params: p }).then((r) => r.data),

  // 高危
  listRisks: () => request.get('/risks').then((r) => r.data),
  createRisk: (d: any) => request.post('/risks', d).then((r) => r.data),
  updateRisk: (id: any, d: any) => request.put(`/risks/${id}`, d).then((r) => r.data),
  deleteRisk: (id: any) => request.delete(`/risks/${id}`).then((r) => r.data),

  // 脚本
  listScripts: (p: any) => request.get('/scripts', { params: p }).then((r) => r.data),
  getScript: (id: any) => request.get(`/scripts/${id}`).then((r) => r.data),
  createScript: (d: any) => request.post('/scripts', d).then((r) => r.data),
  updateScript: (id: any, d: any) => request.put(`/scripts/${id}`, d).then((r) => r.data),
  deleteScript: (id: any) => request.delete(`/scripts/${id}`).then((r) => r.data),
  uploadScript: (fd: any) => request.post('/scripts/upload', fd).then((r) => r.data),

  // 模板
  listTemplates: () => request.get('/templates').then((r) => r.data),
  createTemplate: (d: any) => request.post('/templates', d).then((r) => r.data),
  updateTemplate: (id: any, d: any) => request.put(`/templates/${id}`, d).then((r) => r.data),
  deleteTemplate: (id: any) => request.delete(`/templates/${id}`).then((r) => r.data),

  // 执行
  listExecutions: (p: any) => request.get('/executions', { params: p }).then((r) => r.data),
  quickExec: (d: any) => request.post('/executions/quick', d).then((r) => r.data),

  // 定时任务
  listSchedules: () => request.get('/schedules').then((r) => r.data),
  createSchedule: (d: any) => request.post('/schedules', d).then((r) => r.data),
  updateSchedule: (id: any, d: any) => request.put(`/schedules/${id}`, d).then((r) => r.data),
  deleteSchedule: (id: any) => request.delete(`/schedules/${id}`).then((r) => r.data),
  toggleSchedule: (id: any) => request.post(`/schedules/${id}/toggle`).then((r) => r.data),
  runSchedule: (id: any) => request.post(`/schedules/${id}/run`).then((r) => r.data),

  // 操作审批流
  listApprovals: (p: any) => request.get('/approvals', { params: p }).then((r) => r.data),
  createApproval: (d: any) => request.post('/approvals', d).then((r) => r.data),
  getApproval: (id: any) => request.get(`/approvals/${id}`).then((r) => r.data),
  approveApproval: (id: any) => request.post(`/approvals/${id}/approve`).then((r) => r.data),
  rejectApproval: (id: any, d: any) => request.post(`/approvals/${id}/reject`, d).then((r) => r.data),
  cancelApproval: (id: any) => request.post(`/approvals/${id}/cancel`).then((r) => r.data),

  // 仪表盘
  dashboard: () => request.get('/dashboard').then((r) => r.data),
}

export default api
