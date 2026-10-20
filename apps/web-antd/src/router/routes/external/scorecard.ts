import type { RouteRecordRaw } from 'vue-router';

const BasicLayout = () => import('#/layouts/basic.vue');
const RoomDetailPage = () => import('#/views/scorecard/RoomDetailPage.vue');

/** scorecard 房间详情子页：backend accessMode 路由只由菜单表生成，菜单表按计划不收 detail 行，
 * 故走 externalRoutes 静态注册（不入菜单、保留登录守卫；数据权限由 admin API casbin 强制）。 */
const routes: RouteRecordRaw[] = [
  {
    component: BasicLayout,
    name: 'ScorecardRoomDetailLayout',
    path: '/scorecard/rooms/detail',
    meta: { hideInMenu: true, title: '房间详情' },
    children: [
      {
        component: RoomDetailPage,
        name: 'ScorecardRoomDetail',
        path: '',
        meta: { hideInMenu: true, title: '房间详情' },
      },
    ],
  },
];

export default routes;
