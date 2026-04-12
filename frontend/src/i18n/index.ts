import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import enUS from './resources/en-US'
import zhCN from './resources/zh-CN'

const SHELL_STORAGE_KEY = 'cdnmanage-shell'
export const LANGUAGE_STORAGE_KEY = 'cdnmanage-language'
const SUPPORTED_LANGUAGES = new Set(['zh-CN', 'en-US'])

const resolveStoredLanguage = (): string => {
  if (typeof window === 'undefined' || !window.localStorage) {
    return 'zh-CN'
  }

  const raw = window.localStorage.getItem(SHELL_STORAGE_KEY)
  if (!raw) {
    return 'zh-CN'
  }

  try {
    const parsed = JSON.parse(raw) as {
      state?: { language?: string }
    }
    const storedLanguage = parsed?.state?.language
    if (storedLanguage && SUPPORTED_LANGUAGES.has(storedLanguage)) {
      return storedLanguage
    }
  } catch {
    return 'zh-CN'
  }

  return 'zh-CN'
}

void i18n.use(initReactI18next).init({
  resources: {
    'zh-CN': { translation: zhCN },
    'en-US': { translation: enUS },
  },
  lng: resolveStoredLanguage(),
  fallbackLng: 'zh-CN',
  supportedLngs: ['zh-CN', 'en-US'],
  interpolation: {
    escapeValue: false,
  },
  returnEmptyString: false,
  saveMissing: true,
  missingKeyHandler: (lng, _ns, key) => {
    const language = Array.isArray(lng) ? lng[0] : lng
    console.warn(`[i18n] missing key "${key}" for language "${language}"`)
  },
})

export default i18n
