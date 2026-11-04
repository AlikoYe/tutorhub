import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/', name: 'catalogue', component: () => import('@/views/CatalogueView.vue'), meta: { tab: 'catalogue' } },
  { path: '/specialists/:id(\\d+)', name: 'specialist', component: () => import('@/views/SpecialistView.vue'), props: true },
  { path: '/lessons', name: 'lessons', component: () => import('@/views/LessonsView.vue'), meta: { tab: 'lessons' } },
  { path: '/favourites', name: 'favourites', component: () => import('@/views/FavouritesView.vue'), meta: { tab: 'favourites' } },
  { path: '/teach', name: 'teach', component: () => import('@/views/TeachView.vue'), meta: { tab: 'teach' } },
  { path: '/teach/profile', name: 'teach-profile', component: () => import('@/views/ProfileEditorView.vue') },
  { path: '/teach/schedule', name: 'teach-schedule', component: () => import('@/views/teach/ScheduleView.vue') },
  { path: '/teach/lessons', name: 'teach-lessons', component: () => import('@/views/teach/TeacherLessonsView.vue') },
  { path: '/teach/students', name: 'teach-students', component: () => import('@/views/teach/StudentsView.vue') },
  { path: '/teach/stats', name: 'teach-stats', component: () => import('@/views/teach/StatsView.vue') },
  { path: '/teach/subscription', name: 'teach-subscription', component: () => import('@/views/teach/SubscriptionView.vue') },
  { path: '/teach/verification', name: 'teach-verification', component: () => import('@/views/teach/VerificationView.vue') },
  { path: '/admin', name: 'admin', component: () => import('@/views/AdminView.vue'), meta: { tab: 'admin' } },
  { path: '/admin/queue', name: 'admin-queue', component: () => import('@/views/admin/QueueView.vue') },
  { path: '/admin/dashboard', name: 'admin-dashboard', component: () => import('@/views/admin/DashboardView.vue') },
  { path: '/admin/subscriptions', name: 'admin-subscriptions', component: () => import('@/views/admin/SubscriptionsView.vue') },
  { path: '/admin/reports', name: 'admin-reports', component: () => import('@/views/admin/ReportsView.vue') },
  { path: '/admin/reviews', name: 'admin-reviews', component: () => import('@/views/admin/ReviewsView.vue') },
  { path: '/admin/verifications', name: 'admin-verifications', component: () => import('@/views/admin/VerificationsView.vue') },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('@/views/NotFoundView.vue') },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})
