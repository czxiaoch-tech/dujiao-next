import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { usePageSeo } from './usePageSeo'

/**
 * 关于页共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/About.vue 的行为，仅抽离为 composable。
 */
export function useAbout() {
  const { t, locale } = useI18n()
  const appStore = useAppStore()

  usePageSeo({
    title: () => t('nav.about'),
    canonicalPath: () => '/about',
  })

  const aboutConfig = computed(() => appStore.config?.about || null)
  const contactConfig = computed(() => appStore.config?.contact || null)

  const resolveLocalizedText = (raw: unknown): string => {
    if (!raw || typeof raw !== 'object') {
      return ''
    }

    const record = raw as Record<string, unknown>
    const lang = String(locale.value || appStore.locale || 'zh-CN')
    const candidates = [record[lang], record['zh-CN'], record['zh-TW'], record['en-US']]

    for (const candidate of candidates) {
      if (typeof candidate === 'string' && candidate.trim() !== '') {
        return candidate.trim()
      }
    }

    return ''
  }

  const heroTitle = computed(() => resolveLocalizedText(aboutConfig.value?.hero?.title) || '关于本站')
  const heroSubtitle = computed(() => resolveLocalizedText(aboutConfig.value?.hero?.subtitle) || '独立第三方数字订阅服务站')
  const introductionText = computed(() => resolveLocalizedText(aboutConfig.value?.introduction) || '本站提供数字订阅商品展示、产品码兑换与人工履约服务。当前 ChatGPT Plus 已完成真实支付、自动发码、兑换与履约链路验证。本站并非 OpenAI 官方网站，也不代表 OpenAI。')
  const servicesTitle = computed(() => resolveLocalizedText(aboutConfig.value?.services?.title) || '我们提供什么')
  const contactTitle = computed(() => resolveLocalizedText(aboutConfig.value?.contact?.title) || '联系与售后')
  const contactText = computed(() => resolveLocalizedText(aboutConfig.value?.contact?.text) || '订单、产品码或履约状态有问题，可通过页面公开联系方式联系我们。')

  const serviceItems = computed(() => {
    const raw = aboutConfig.value?.services?.items
    if (!Array.isArray(raw) || raw.length === 0) {
      return [
        '公开展示实际可售商品与价格',
        '付款后自动获取产品码',
        '产品码兑换后进入人工履约',
        '个人中心查看订单与履约状态',
      ]
    }

    return raw
      .map((item) => resolveLocalizedText(item))
      .filter((item) => item !== '')
  })

  const hasIntroduction = computed(() => introductionText.value !== '')
  const hasServices = computed(() => servicesTitle.value !== '' || serviceItems.value.length > 0)
  const supportEmail = '527821823@qq.com'
  const hasContactLinks = computed(() => !!(contactConfig.value?.telegram || contactConfig.value?.whatsapp || supportEmail))
  const hasContact = computed(() => contactTitle.value !== '' || contactText.value !== '' || hasContactLinks.value)

  onMounted(async () => {
    if (!appStore.config) {
      await appStore.loadConfig()
    }
  })

  return {
    contactConfig,
    supportEmail,
    heroTitle,
    heroSubtitle,
    introductionText,
    servicesTitle,
    contactTitle,
    contactText,
    serviceItems,
    hasIntroduction,
    hasServices,
    hasContactLinks,
    hasContact,
  }
}
