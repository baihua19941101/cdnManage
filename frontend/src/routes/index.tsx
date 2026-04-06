import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'

import { AppProviders } from '../app/AppProviders'
import { AppShell } from '../layouts/AppShell'
import { DashboardPage } from '../pages/DashboardPage'
import { LoginPage } from '../pages/LoginPage'
import { NotFoundPage } from '../pages/NotFoundPage'
import { ProjectsPage } from '../pages/ProjectsPage'
import { ResourcePage } from '../pages/ResourcePage'
import { SetupPage } from '../pages/SetupPage'
import { UnauthorizedPage } from '../pages/UnauthorizedPage'
import { UsersPage } from '../pages/UsersPage'
import { useAuthStore } from '../store/auth'

export function AppRouter() {
  const isLoggedIn = useAuthStore((state) => state.isLoggedIn)

  return (
    <BrowserRouter>
      <AppProviders>
        <Routes>
          <Route
            path="/login"
            element={isLoggedIn ? <Navigate to="/" replace /> : <LoginPage />}
          />
          <Route path="/setup" element={<SetupPage />} />
          <Route path="/unauthorized" element={<UnauthorizedPage />} />

          <Route
            path="/"
            element={isLoggedIn ? <AppShell /> : <Navigate to="/login" replace />}
          >
            <Route index element={<DashboardPage />} />
            <Route path="projects" element={<ProjectsPage />} />
            <Route path="users" element={<UsersPage />} />
            <Route
              path="storage"
              element={
                <ResourcePage
                  title="Storage"
                  description="Bucket object workflows and audit shortcuts will land in task 12.1."
                />
              }
            />
            <Route
              path="cdn"
              element={
                <ResourcePage
                  title="CDN"
                  description="Refresh and sync operation pages will land in task 12.2."
                />
              }
            />
            <Route
              path="audits"
              element={
                <ResourcePage
                  title="Audits"
                  description="Platform and project audit search pages will land in task 12.3."
                />
              }
            />
          </Route>

          <Route path="/home" element={<Navigate to="/" replace />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </AppProviders>
    </BrowserRouter>
  )
}
