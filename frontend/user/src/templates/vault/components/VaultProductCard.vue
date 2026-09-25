<template>
  <RouterLink
    :to="`/products/${product.slug}`"
    class="vault-shop-card"
    :class="{ 'opacity-[0.68]': soldOut }"
  >
    <div class="vault-shop-card-media" :class="coverClass">
      <img
        v-if="coverImage"
        :src="coverImage"
        :alt="title"
        loading="lazy"
        class="absolute inset-0 h-full w-full object-cover"
        @error="imageErrored = true"
      />
      <Package v-else class="relative z-[1] h-12 w-12 text-white/75" />
    </div>

    <div class="flex flex-1 flex-col p-4">
      <h3 class="line-clamp-2 min-h-[44px] text-[17px] font-black leading-[1.3] tracking-[-0.02em]">
        {{ title }}
      </h3>

      <div class="mt-3 vault-shop-price">
        {{ formatPrice(displayPrice, siteCurrency) }}
      </div>

      <div class="mt-3 flex flex-wrap gap-2">
        <span class="vault-shop-status bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]">
          <component :is="product.fulfillment_type === 'auto' ? Zap : Pencil" class="h-3.5 w-3.5" />
          {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
        </span>
        <span class="vault-shop-status" :class="stockPill.tone">
          <component :is="stockPill.icon" class="h-3.5 w-3.5" />
          {{ stockPill.label }}
        </span>
      </div>

      <button
        v-if="!soldOut"
        type="button"
        class="vault-shop-buy mt-4"
        :aria-label="t('products.quickBuyAria')"
        @click.prevent.stop="$emit('quickBuy', product)"
      >
        {{ t('products.quickBuy') }}
        <ArrowRight class="h-4 w-4" />
      </button>
      <span v-else class="vault-shop-buy mt-4 cursor-not-allowed opacity-45">
        {{ t('products.stockStatus.outOfStock') }}
      </span>
    </div>
  </RouterLink>
</template>

<script setup lang="ts">
import { computed, ref, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlarmClock, ArrowRight, Package, Pencil, XCircle, Zap } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../../../utils/image'
import { useLocalized, useProductLabels } from '../../../composables/useProduct'

const props = withDefaults(defineProps<{ product: any; index?: number }>(), { index: 0 })

defineEmits<{ quickBuy: [product: any] }>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const {
  getStockStatusLabel,
  getFulfillmentTypeLabel,
  isSoldOut,
  hasPromotionPrice,
  getPromotionPriceAmount,
} = useProductLabels()

const covers = [
  'bg-[#161b20]',
  'bg-[#22313a]',
  'bg-[#283327]',
  'bg-[#312d26]',
  'bg-[#2d2936]',
]
const coverClass = computed(() => covers[(props.index ?? 0) % covers.length])

const title = computed(() => getLocalizedText(props.product?.title))
const soldOut = computed(() => isSoldOut(props.product))
const promo = computed(() => hasPromotionPrice(props.product))
const displayPrice = computed(() => promo.value ? getPromotionPriceAmount(props.product) : props.product.price_amount)

const imageErrored = ref(false)
const coverImage = computed(() => {
  if (imageErrored.value) return ''
  const primary = getFirstImageUrl(props.product?.images)
  if (primary) return primary
  const icon = props.product?.category?.icon
  return icon ? getImageUrl(icon) : ''
})

const stockPill = computed<{ tone: string; icon: Component; label: string }>(() => {
  if (soldOut.value) {
    return { tone: 'bg-secondary text-muted-foreground', icon: XCircle, label: t('products.stockStatus.outOfStock') }
  }
  if (props.product?.stock_status === 'low_stock') {
    return { tone: 'bg-[color:var(--gold-soft)] text-[color:var(--gold-strong)]', icon: AlarmClock, label: getStockStatusLabel(props.product) }
  }
  return { tone: 'bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]', icon: Zap, label: getStockStatusLabel(props.product) }
})
</script>
