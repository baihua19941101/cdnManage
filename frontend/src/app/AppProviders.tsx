import type { PropsWithChildren } from 'react'
import { useEffect } from 'react'
import { App as AntdApp, ConfigProvider } from 'antd'
import { useLocation } from 'react-router-dom'

import { themePresets } from './themes'
import { useShellStore } from '../store/shell'

export function AppProviders({ children }: PropsWithChildren) {
  const location = useLocation()
  const themeMode = useShellStore((state) => state.themeMode)
  const resolvedTheme = location.pathname === '/login' || location.pathname === '/setup' ? 'auth' : themeMode

  useEffect(() => {
    document.documentElement.dataset.theme = resolvedTheme
  }, [resolvedTheme])

  return (
    <ConfigProvider theme={themePresets[resolvedTheme].theme}>
      <AntdApp>{children}</AntdApp>
    </ConfigProvider>
  )
}
