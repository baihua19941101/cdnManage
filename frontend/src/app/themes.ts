import { theme } from 'antd'
import type { ThemeConfig } from 'antd'

export type ThemeMode = 'light' | 'auth' | 'dark'

type ThemePreset = {
  label: string
  theme: ThemeConfig
  surfaceClassName: string
}

const sharedTypography = {
  fontFamily: '"Trebuchet MS", "Lucida Sans Unicode", "Lucida Grande", sans-serif',
}

export const themePresets: Record<ThemeMode, ThemePreset> = {
  light: {
    label: 'Light',
    surfaceClassName: 'theme-light',
    theme: {
      algorithm: theme.defaultAlgorithm,
      token: {
        ...sharedTypography,
        colorPrimary: '#14708c',
        colorBgBase: '#edf5f7',
        colorBgContainer: '#ffffff',
        colorTextBase: '#17303b',
        colorBorderSecondary: 'rgba(20, 112, 140, 0.16)',
        borderRadius: 20,
      },
    },
  },
  auth: {
    label: 'Auth',
    surfaceClassName: 'theme-auth',
    theme: {
      algorithm: theme.defaultAlgorithm,
      token: {
        ...sharedTypography,
        colorPrimary: '#9c5b1f',
        colorBgBase: '#f4ebdc',
        colorBgContainer: '#fffaf2',
        colorTextBase: '#2d2118',
        colorBorderSecondary: 'rgba(156, 91, 31, 0.18)',
        borderRadius: 24,
      },
    },
  },
  dark: {
    label: 'Dark',
    surfaceClassName: 'theme-dark',
    theme: {
      algorithm: theme.darkAlgorithm,
      token: {
        ...sharedTypography,
        colorPrimary: '#42d6c8',
        colorBgBase: '#07131b',
        colorBgContainer: '#0d1f29',
        colorTextBase: '#d9ebf2',
        colorBorderSecondary: 'rgba(125, 176, 196, 0.22)',
        borderRadius: 20,
      },
    },
  },
}
