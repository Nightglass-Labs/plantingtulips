import { render } from '@solidjs/web';
import { createRouter, defineRoutes } from '@solidjs/router';
import './index.css';
import { AppShell } from './ui/AppShell';
import { HomePage } from './pages/HomePage';
import { CategoriesPage } from './pages/CategoriesPage';
import { CategoryPage } from './pages/CategoryPage';
import { TagPage } from './pages/TagPage';
import { LibraryPage } from './pages/LibraryPage';
import { VideoPage } from './pages/VideoPage';
import { ReportPage } from './pages/ReportPage';
import { AdminDashboardPage } from './pages/AdminDashboardPage';
import { AdminReportsPage } from './pages/AdminReportsPage';
import { LegalPage } from './pages/LegalPage';

const root = document.getElementById('root');
if (!root) throw new Error('#root was not found');

const routes = defineRoutes([
  {
    component: AppShell,
    children: [
      { path: '/', component: HomePage },
      { path: '/categories', component: CategoriesPage },
      { path: '/category/:slug', component: CategoryPage },
      { path: '/tag/:slug', component: TagPage },
      { path: '/library', component: LibraryPage },
      { path: '/watch/:provider/:id', component: VideoPage },
      { path: '/report', component: ReportPage },
      { path: '/admin', component: AdminDashboardPage },
      { path: '/admin/reports', component: AdminReportsPage },
      { path: '/:document', component: LegalPage },
    ],
  },
]);

const AppRouter = createRouter({ routes });
render(() => <AppRouter />, root);
