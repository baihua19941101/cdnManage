import { theme } from 'antd'
import type { ThemeConfig } from 'antd'

export type ThemeMode = 'light' | 'dark'

type ThemePreset = {
  label: string
  theme: ThemeConfig
  surfaceClassName: string
}

const sharedToken = {
  fontFamily: '"Inter", "Segoe UI Variable", "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
  colorInfo: '#669dff',
  colorSuccess: '#3ddfb1',
  colorWarning: '#f2bf5c',
  colorError: '#ff716c',
  colorLink: '#81ecff',
  borderRadius: 12,
  borderRadiusLG: 16,
  borderRadiusSM: 8,
  controlHeight: 38,
  controlHeightLG: 44,
  controlOutlineWidth: 2,
  wireframe: false,
}

const sharedComponents: NonNullable<ThemeConfig['components']> = {
  Layout: {
    bodyBg: 'transparent',
    headerBg: 'transparent',
    siderBg: 'transparent',
  },
  Card: {
    bodyPadding: 20,
  },
  Button: {
    borderRadius: 12,
    controlHeight: 38,
    controlHeightLG: 44,
    fontWeight: 600,
  },
  Menu: {
    itemBg: 'transparent',
    activeBarHeight: 0,
    itemBorderRadius: 10,
    itemHeight: 44,
    itemMarginInline: 10,
    itemMarginBlock: 6,
  },
  Input: {
    activeBorderColor: '#81ecff',
    hoverBorderColor: '#8bd7ff',
  },
  Segmented: {
    trackBg: 'rgba(129, 236, 255, 0.1)',
  },
}

export const themePresets: Record<ThemeMode, ThemePreset> = {
  light: {
    label: '浅色',
    surfaceClassName: 'theme-light',
    theme: {
      algorithm: theme.defaultAlgorithm,
      token: {
        ...sharedToken,
        colorPrimary: '#0f7f91',
        colorBgBase: '#eff6fb',
        colorBgLayout: '#e8f1f8',
        colorBgContainer: '#f8fcff',
        colorBgElevated: '#ffffff',
        colorText: '#2c4553',
        colorTextHeading: '#2f5365',
        colorTextSecondary: '#637786',
        colorBorder: '#bfd2df',
        colorBorderSecondary: '#d7e3ec',
        colorFillSecondary: 'rgba(19, 45, 62, 0.05)',
        boxShadow: '0 20px 42px rgba(41, 85, 110, 0.14)',
      },
      components: {
        ...sharedComponents,
        Button: {
          ...sharedComponents.Button,
          primaryShadow: '0 12px 26px rgba(15, 127, 145, 0.26)',
        },
        Menu: {
          ...sharedComponents.Menu,
          itemSelectedBg: 'rgba(15, 127, 145, 0.12)',
          itemSelectedColor: '#0b3f49',
          itemColor: '#325361',
        },
        Table: {
          headerBg: '#eef4f9',
          headerColor: '#315364',
          rowHoverBg: '#f7fbff',
        },
      },
    },
  },
  dark: {
    label: '深色',
    surfaceClassName: 'theme-dark',
    theme: {
      algorithm: theme.darkAlgorithm,
      token: {
        ...sharedToken,
        colorPrimary: '#81ecff',
        colorBgBase: '#0a0e14',
        colorBgLayout: '#0a0e14',
        colorBgContainer: '#151a21',
        colorBgElevated: '#1b2028',
        colorText: '#f1f3fc',
        colorTextHeading: '#e8f6ff',
        colorTextSecondary: '#a8abb3',
        colorBorder: '#2a323c',
        colorBorderSecondary: '#222a34',
        colorFillSecondary: 'rgba(168, 171, 179, 0.12)',
        boxShadow: '0 24px 48px rgba(3, 8, 13, 0.52)',
      },
      components: {
        ...sharedComponents,
        Button: {
          ...sharedComponents.Button,
          primaryShadow: '0 14px 32px rgba(129, 236, 255, 0.24)',
        },
        Menu: {
          ...sharedComponents.Menu,
          itemSelectedBg: 'rgba(129, 236, 255, 0.12)',
          itemSelectedColor: '#b9f4ff',
          itemColor: '#9ea4b0',
        },
        Table: {
          headerBg: '#171c23',
          headerColor: '#98a3b4',
          rowHoverBg: '#131921',
        },
      },
    },
  },
}
