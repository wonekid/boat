import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import Layout from '@/layout/index.vue'

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'Login', component: () => import('@/views/login/index.vue'), meta: { public: true } },
  {
    path: '/',
    component: Layout,
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'Dashboard', component: () => import('@/views/dashboard/index.vue'), meta: { title: '仪表盘', icon: 'Odometer' } },
      { path: 'system/user', name: 'User', component: () => import('@/views/system/user.vue'), meta: { title: '用户管理', perm: 'system:user:list' } },
      { path: 'system/role', name: 'Role', component: () => import('@/views/system/role.vue'), meta: { title: '角色管理', perm: 'system:role:list' } },
      { path: 'system/menu', name: 'Menu', component: () => import('@/views/system/menu.vue'), meta: { title: '菜单管理', perm: 'system:menu:list' } },
      { path: 'system/dept', name: 'Dept', component: () => import('@/views/system/dept.vue'), meta: { title: '部门管理', perm: 'system:dept:list' } },
      { path: 'asset/host', name: 'Host', component: () => import('@/views/asset/host.vue'), meta: { title: '主机管理', perm: 'asset:host:list' } },
      { path: 'asset/credential', name: 'Credential', component: () => import('@/views/asset/credential.vue'), meta: { title: '凭证管理', perm: 'asset:credential:list' } },
      { path: 'asset/group', name: 'HostGroup', component: () => import('@/views/asset/group.vue'), meta: { title: '主机分组', perm: 'asset:group:list' } },
      { path: 'asset/auth', name: 'Auth', component: () => import('@/views/asset/auth.vue'), meta: { title: '授权管理', perm: 'asset:auth:list' } },
      { path: 'ops/terminal', name: 'Terminal', component: () => import('@/views/ops/terminal.vue'), meta: { title: 'Web终端', perm: 'ops:terminal:connect' } },
      { path: 'ops/session', name: 'Session', component: () => import('@/views/ops/session.vue'), meta: { title: '会话审计', perm: 'ops:session:list' } },
      { path: 'ops/replay/:id', name: 'Replay', component: () => import('@/views/ops/replay.vue'), meta: { title: '录像回放', perm: 'ops:session:list' } },
      { path: 'ops/audit', name: 'Audit', component: () => import('@/views/ops/audit.vue'), meta: { title: '操作日志', perm: 'ops:audit:list' } },
      { path: 'security/risk', name: 'Risk', component: () => import('@/views/security/risk.vue'), meta: { title: '高危指令', perm: 'security:risk:list' } },
      { path: 'task/script', name: 'Script', component: () => import('@/views/task/script.vue'), meta: { title: '脚本库', perm: 'task:script:list' } },
      { path: 'task/template', name: 'Template', component: () => import('@/views/task/template.vue'), meta: { title: '任务模板', perm: 'task:template:list' } },
      { path: 'task/execution', name: 'Execution', component: () => import('@/views/task/execution.vue'), meta: { title: '任务执行', perm: 'task:execution:list' } },
      { path: 'task/schedule', name: 'Schedule', component: () => import('@/views/task/schedule.vue'), meta: { title: '定时任务', perm: 'task:schedule:list' } },
      { path: 'task/approval', name: 'Approval', component: () => import('@/views/task/approval.vue'), meta: { title: '操作审批', perm: 'task:approval:approve' } },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('boat_token')
  if (to.meta.public) {
    next()
    return
  }
  if (!token) {
    next('/login')
    return
  }
  next()
})

export default router
