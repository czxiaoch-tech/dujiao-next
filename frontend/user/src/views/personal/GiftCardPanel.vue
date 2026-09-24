<template>
  <div class="space-y-6 gift-card-panel-enter">
    <div class="rounded-2xl border bg-card p-7 shadow-sm">
      <PanelHeading
        :title="t('personalCenter.giftCard.title')"
        :description="t('personalCenter.giftCard.subtitle')"
        :icon="Gift"
      >
        <template #actions>
          <Badge variant="accent" size="sm">{{ t('personalCenter.tabs.giftCard') }}</Badge>
        </template>
      </PanelHeading>

      <Alert v-if="panelAlert" class="mb-5" :variant="pageAlertVariant(panelAlert.level)" :class="pageAlertToneClass(panelAlert.level)">
        <AlertDescription>{{ panelAlert.message }}</AlertDescription>
      </Alert>

      <div
        v-if="lastRedeem"
        class="mb-6 rounded-2xl border border-success/25 bg-success/10 p-4 shadow-sm success-burst"
      >
        <div class="flex items-start gap-3">
          <div class="mt-0.5 flex h-7 w-7 items-center justify-center rounded-full bg-success text-white">
            <Check class="h-4 w-4" />
          </div>
          <div class="flex-1 space-y-2">
            <h3 class="text-sm font-semibold text-success">
              {{ lastRedeem.order ? t('personalCenter.giftCard.productSuccessTitle') : t('personalCenter.giftCard.successTitle') }}
            </h3>
            <div v-if="lastRedeem.order" class="grid grid-cols-1 gap-2 text-xs text-success/90 md:grid-cols-2">
              <div>
                <div class="opacity-75">{{ t('personalCenter.giftCard.successCode') }}</div>
                <div class="mt-0.5 font-mono">{{ String(lastRedeem.gift_card?.code || '-').toUpperCase() }}</div>
              </div>
              <div>
                <div class="opacity-75">{{ t('personalCenter.giftCard.productOrderNo') }}</div>
                <div class="mt-0.5 font-mono">{{ lastRedeem.order.order_no }}</div>
              </div>
            </div>
            <div v-else class="grid grid-cols-1 gap-2 text-xs text-success/90 md:grid-cols-3">
              <div>
                <div class="opacity-75">{{ t('personalCenter.giftCard.successCode') }}</div>
                <div class="mt-0.5 font-mono">{{ String(lastRedeem.gift_card?.code || '-').toUpperCase() }}</div>
              </div>
              <div>
                <div class="opacity-75">{{ t('personalCenter.giftCard.successAmount') }}</div>
                <div class="mt-0.5 font-semibold">{{ redeemedAmountText }}</div>
              </div>
              <div>
                <div class="opacity-75">{{ t('personalCenter.giftCard.successBalance') }}</div>
                <div class="mt-0.5 font-semibold">{{ currentBalanceText }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <form class="space-y-5" @submit.prevent="submit">
        <div>
          <Label class="mb-2 block">{{ t('personalCenter.giftCard.codeLabel') }}</Label>
          <Input
            :model-value="redeemCode"
            type="text"
            maxlength="80"
            autocomplete="off"
            :placeholder="t('personalCenter.giftCard.codePlaceholder')"
            class="h-11 font-mono uppercase tracking-[0.08em]"
            @update:model-value="handleCodeInput"
          />
        </div>

        <div v-if="resolved?.redeem_type === 'product' && resolved.product" class="rounded-2xl border bg-secondary/40 p-4">
          <div class="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">
            {{ t('personalCenter.giftCard.productTargetLabel') }}
          </div>
          <div class="mt-1 text-base font-semibold text-foreground">{{ productTitle }}</div>
        </div>

        <CheckoutManualForm
          v-if="productManualFormProducts.length"
          v-model="manualFormData"
          :manual-form-products="productManualFormProducts"
          :submit-attempted="submitAttempted"
          :get-manual-field-label="getManualFieldLabel"
          :get-manual-field-placeholder="getManualFieldPlaceholder"
          :manual-field-error="manualFieldError"
        />

        <div v-if="resolved && redeemCaptchaEnabled" class="rounded-xl border px-4 py-3">
          <p class="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">{{ t('auth.common.captchaLabel') }}</p>
          <div class="mt-2">
            <ImageCaptcha
              v-if="captchaProvider === 'image'"
              ref="imageCaptchaRef"
              v-model="captchaPayload"
              :disabled="submitting"
              @config-stale="handleCaptchaConfigStale"
            />
            <TurnstileCaptcha
              v-else-if="captchaProvider === 'turnstile'"
              ref="turnstileRef"
              v-model="turnstileToken"
              :site-key="turnstileSiteKey"
            />
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-3 pt-1">
          <Button type="submit" :disabled="submitting || resolving" class="h-11 px-5 font-bold">
            {{
              resolving
                ? t('personalCenter.giftCard.resolving')
                : submitting
                  ? t('personalCenter.giftCard.redeeming')
                  : resolved
                    ? t('personalCenter.giftCard.redeemButton')
                    : t('personalCenter.giftCard.resolveButton')
            }}
          </Button>
          <Button type="button" variant="outline" :disabled="submitting || resolving" class="h-11 font-semibold" @click="resetForm">
            {{ t('personalCenter.giftCard.resetButton') }}
          </Button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  giftCardAPI,
  type CaptchaPayload,
  type GiftCardRedeemResult,
  type GiftCardResolveResult,
} from '../../api'
import { useAppStore } from '../../stores/app'
import { useLocalized } from '../../composables/useProduct'
import { pageAlertVariant, pageAlertToneClass, type PageAlert } from '../../utils/alerts'
import ImageCaptcha from '../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../components/captcha/TurnstileCaptcha.vue'
import CheckoutManualForm from '../../components/checkout/CheckoutManualForm.vue'
import { Check, Gift } from 'lucide-vue-next'
import PanelHeading from '../../components/shared/PanelHeading.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface ManualField {
  key: string
  type: string
  required: boolean
  label?: Record<string, string>
  placeholder?: Record<string, string>
  regex?: string
  min?: number
  max?: number
  max_len?: number
  options: string[]
}

const { t } = useI18n()
const appStore = useAppStore()
const { getLocalizedText } = useLocalized()

const redeemCode = ref('')
const resolvedCode = ref('')
const resolved = ref<GiftCardResolveResult | null>(null)
const resolving = ref(false)
const submitting = ref(false)
const submitAttempted = ref(false)
const panelAlert = ref<PageAlert | null>(null)
const lastRedeem = ref<GiftCardRedeemResult | null>(null)
const manualFormData = ref<Record<string, Record<string, any>>>({})

const captchaPayload = ref<CaptchaPayload>({})
const turnstileToken = ref('')
const imageCaptchaRef = ref<InstanceType<typeof ImageCaptcha> | null>(null)
const turnstileRef = ref<InstanceType<typeof TurnstileCaptcha> | null>(null)

const captchaConfig = computed(() => appStore.config?.captcha || null)
const captchaProvider = computed(() => String(captchaConfig.value?.provider || 'none'))
const redeemCaptchaEnabled = computed(() => {
  return !!captchaConfig.value?.scenes?.gift_card_redeem && captchaProvider.value !== 'none'
})
const turnstileSiteKey = computed(() => String(captchaConfig.value?.turnstile?.site_key || ''))

const productTitle = computed(() => getLocalizedText(resolved.value?.product?.title || {}))

const productFields = computed<ManualField[]>(() => {
  const raw = resolved.value?.product?.manual_form_schema?.fields
  if (!Array.isArray(raw)) return []
  return raw
    .map((field: any) => ({
      key: String(field?.key || '').trim(),
      type: String(field?.type || 'text').trim(),
      required: Boolean(field?.required),
      label: field?.label || undefined,
      placeholder: field?.placeholder || undefined,
      regex: String(field?.regex || '').trim() || undefined,
      min: typeof field?.min === 'number' ? field.min : undefined,
      max: typeof field?.max === 'number' ? field.max : undefined,
      max_len: typeof field?.max_len === 'number' ? field.max_len : undefined,
      options: Array.isArray(field?.options) ? field.options.map((item: any) => String(item)) : [],
    }))
    .filter((field: ManualField) => Boolean(field.key))
})

const productManualFormProducts = computed(() => {
  const product = resolved.value?.product
  if (!product || productFields.value.length === 0) return []
  return [{
    itemKey: String(product.product_id),
    productId: product.product_id,
    title: product.title,
    fields: productFields.value,
    skuCount: 1,
  }]
})

const redeemedAmountText = computed(() => {
  const rawAmount = String(lastRedeem.value?.wallet_delta || lastRedeem.value?.gift_card?.amount || '').trim()
  const currency = String(lastRedeem.value?.gift_card?.currency || appStore.config?.currency || 'CNY').trim()
  return rawAmount ? `${rawAmount} ${currency}` : '-'
})

const currentBalanceText = computed(() => {
  const balance = String(lastRedeem.value?.wallet?.balance || '').trim()
  const currency = String(lastRedeem.value?.gift_card?.currency || appStore.config?.currency || 'CNY').trim()
  return balance ? `${balance} ${currency}` : '-'
})

const getManualFieldLabel = (field: ManualField) => getLocalizedText(field.label || {}) || field.key
const getManualFieldPlaceholder = (field: ManualField) => getLocalizedText(field.placeholder || {})

const manualFieldError = (itemKey: string, fieldKey: string) => {
  const field = productFields.value.find((item) => item.key === fieldKey)
  if (!field || !field.required) return ''
  const value = manualFormData.value[itemKey]?.[fieldKey]
  const empty = Array.isArray(value) ? value.length === 0 : String(value ?? '').trim() === ''
  return empty ? t('checkout.manualFormFieldRequired', { name: getManualFieldLabel(field) }) : ''
}

const productFormValid = () => {
  const product = resolved.value?.product
  if (!product) return true
  const key = String(product.product_id)
  return productFields.value.every((field) => !manualFieldError(key, field.key))
}

const getCaptchaPayload = (): CaptchaPayload | undefined => {
  if (!redeemCaptchaEnabled.value) return undefined
  if (captchaProvider.value === 'image') {
    return {
      captcha_id: captchaPayload.value.captcha_id || '',
      captcha_code: captchaPayload.value.captcha_code || '',
    }
  }
  if (captchaProvider.value === 'turnstile') {
    return { turnstile_token: turnstileToken.value || '' }
  }
  return undefined
}

const ensureCaptchaPassed = () => {
  if (!redeemCaptchaEnabled.value) return true
  if (captchaProvider.value === 'image') {
    return Boolean(captchaPayload.value.captcha_id && captchaPayload.value.captcha_code)
  }
  if (captchaProvider.value === 'turnstile') {
    return Boolean(turnstileToken.value)
  }
  return false
}

const resetCaptcha = () => {
  captchaPayload.value = {}
  turnstileToken.value = ''
  imageCaptchaRef.value?.refresh()
  turnstileRef.value?.reset()
}

const clearResolved = () => {
  resolved.value = null
  resolvedCode.value = ''
  manualFormData.value = {}
  submitAttempted.value = false
}

const handleCodeInput = (value: string | number) => {
  const next = String(value ?? '')
  if (next !== redeemCode.value) {
    redeemCode.value = next
    clearResolved()
    lastRedeem.value = null
    panelAlert.value = null
  }
}

const resetForm = () => {
  redeemCode.value = ''
  clearResolved()
  panelAlert.value = null
  lastRedeem.value = null
  resetCaptcha()
}

const handleCaptchaConfigStale = async () => {
  await appStore.loadConfig(true)
  resetCaptcha()
}

const resolveCode = async () => {
  const code = redeemCode.value.trim().toUpperCase()
  if (!code) {
    panelAlert.value = { level: 'warning', message: t('personalCenter.giftCard.errors.codeRequired') }
    return false
  }
  resolving.value = true
  panelAlert.value = null
  try {
    const response = await giftCardAPI.resolve({ code })
    const payload = response.data.data as GiftCardResolveResult
    resolved.value = payload
    resolvedCode.value = code
    manualFormData.value = payload?.product
      ? { [String(payload.product.product_id)]: {} }
      : {}
    return true
  } catch (err: any) {
    clearResolved()
    panelAlert.value = {
      level: 'error',
      message: err?.message || t('personalCenter.giftCard.errors.resolveFailed'),
    }
    return false
  } finally {
    resolving.value = false
  }
}

const submit = async () => {
  panelAlert.value = null
  const code = redeemCode.value.trim().toUpperCase()
  if (!resolved.value || resolvedCode.value !== code) {
    await resolveCode()
    return
  }

  submitAttempted.value = true
  if (resolved.value.redeem_type === 'product' && !productFormValid()) {
    panelAlert.value = {
      level: 'warning',
      message: t('personalCenter.giftCard.errors.formRequired'),
    }
    return
  }
  if (!ensureCaptchaPassed()) {
    panelAlert.value = { level: 'warning', message: t('auth.common.captchaRequired') }
    return
  }

  submitting.value = true
  try {
    const productID = resolved.value.product?.product_id
    const response = await giftCardAPI.redeem({
      code,
      manual_form_data: productID ? (manualFormData.value[String(productID)] || {}) : undefined,
      captcha_payload: getCaptchaPayload(),
    })
    const payload = response.data.data as GiftCardRedeemResult
    lastRedeem.value = payload
    panelAlert.value = {
      level: 'success',
      message: payload.order
        ? t('personalCenter.giftCard.productRedeemSuccess', { orderNo: payload.order.order_no })
        : t('personalCenter.giftCard.redeemSuccess', {
            amount: String(payload.wallet_delta || payload.gift_card?.amount || ''),
            currency: String(payload.gift_card?.currency || appStore.config?.currency || 'CNY'),
          }),
    }
    redeemCode.value = ''
    clearResolved()
    resetCaptcha()
  } catch (err: any) {
    panelAlert.value = {
      level: 'error',
      message: err?.message || t('personalCenter.giftCard.errors.redeemFailed'),
    }
    resetCaptcha()
  } finally {
    submitting.value = false
  }
}

if (!appStore.config) {
  void appStore.loadConfig()
}
</script>

<style scoped>
.gift-card-panel-enter {
  animation: gift-card-panel-enter 0.45s ease both;
}

.success-burst {
  animation: gift-card-success-burst 0.45s ease both;
}

@keyframes gift-card-panel-enter {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes gift-card-success-burst {
  0% {
    opacity: 0;
    transform: translateY(8px) scale(0.98);
  }
  100% {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}
</style>
