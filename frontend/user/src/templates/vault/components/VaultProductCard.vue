<template>
  <RouterLink
    :to="`/products/${product.slug}`"
    class="vault-pass-card"
    :class="{ 'opacity-[0.68]': soldOut }"
  >
    <div class="vault-pass-cover" :class="coverClass">
      <img
        v-if="coverImage"
        :src="coverImage"
        :alt="title"
        loading="lazy"
        class="absolute inset-0 h-full w-full object-cover"
        @error="imageErrored = true"
      />
      <Package v-else class="absolute bottom-5 right-5 z-[2] h-14 w-14 text-white/75" />

      <span class="vault-pass-index">PASS {{ String((index ?? 0) + 1).padStart(2, '0') }}</span>

      <div v-if="!soldOut && product.tags && product.tags.length" class="absolute bottom-3 left-3 z-[3] flex max-w-[82%] flex-wrap gap-1.5">
        <span
          v-for="(tag, i) in product.tags.slice(0, 2)"
          :key="i"
          class="inline-flex max-w-full items-center truncate rounded-[6px] border border-white/15 bg-black/55 px-2 py-1 text-[10px] font-black uppercase tracking-[0.06em] text-white backdrop-blur"
        >{{ tag }}</span>
      </div>
    </div>

    <div class="flex flex-1 flex-col p-4">
      <div>
        <div v-if="categoryName" class="mb-1.5 text-[11px] font-black uppercase tracking-[0.12em] text-muted-foreground">{{ categoryName }}</div>
        <h3 class="line-clamp-2 text-[18px] font-black leading-[1.18] tracking-[-0.025em]">{{ title }}</h3>
      </div>

      <div class="mt-3 flex flex-wrap gap-1.5">
        <span class="inline-flex items-center gap-1 rounded-[6px] bg-[color:var(--teal-soft)] px-2 py-1 text-[10.5px] font-black text-[color:var(--teal-strong)]">
          <component :is="product.fulfillment_type === 'auto' ? Zap : Pencil" class="h-3 w-3" />
          {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
        </span>
        <span class="inline-flex items-center gap-1 rounded-[6px] bg-secondary px-2 py-1 text-[10.5px] font-black text-muted-foreground">
          <component :is="product.purchase_type === 'guest' ? UserPlus : Lock" class="h-3 w-3" />
          {{ getPurchaseTypeLabel(product.purchase_type) }}
        </span>
        <span class="inline-flex items-center gap-1 rounded-[6px] px-2 py-1 text-[10.5px] font-black" :class="stockPill.tone">
          <component :is="stockPill.icon" class="h-3 w-3" />
          {{ stockPill.label }}
        </span>
      </div>

      <div class="vault-pass-meta mt-4">
        <span>AI ACCESS</span>
        <span v-if="priceSignal" :class="priceSignal.tone" class="rounded-[5px] px-2 py-0.5 normal-case tracking-normal">{{ priceSignal.label }}</span>
        <span v-else>READY</span>
      </div>

      <div class="mt-auto flex items-end justify-between gap-3 pt-4">
        <div class="min-w-0">
          <template v-if="promo">
            <div class="vault-pass-price text-foreground">{{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}</div>
            <div class="mt-0.5 text-xs font-bold text-muted-foreground line-through">{{ formatPrice(product.price_amount, siteCurrency) }}</div>
          </template>
          <div v-else class="vault-pass-price text-foreground">{{ formatPrice(product.price_amount, siteCurrency) }}</div>
        </div>

        <span v-if="soldOut" class="inline-flex min-h-[38px] items-center rounded-[8px] border px-3 text-xs font-black text-muted-foreground">
          {{ t('products.stockStatus.outOfStock') }}
        </span>
        <button
          v-else
          type="button"
          class="vault-pass-action"
          :aria-label="t('products.quickBuyAria')"
          @click.prevent.stop="$emit('quickBuy', product)"
        >
          <Zap class="h-3.5 w-3.5" />
          {{ t('products.quickBuy') }}
        </button>
      </div>
    </div>
  </RouterLink>
</template>

<script setup lang="ts">
import { computed, ref, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlarmClock, Lock, Package, Pencil, UserPlus, XCircle, Zap } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../../../utils/image'
import { useLocalized, useProductLabels } from '../../../composables/useProduct'

const props = withDefaults(defineProps<{ product: any; index?: number }>(), { index: 0 })

defineEmits<{ quickBuy: [product: any] }>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const {
  getStockStatusLabel, getPurchaseTypeLabel, getFulfillmentTypeLabel,
  isSoldOut, hasPromotionPrice, getPromotionPriceAmount, hasWholesalePrices, hasPromotionRules,
} = useProductLabels()

// 封面渐变（对应原 cover-red/teal/plum/gold/ink）
const covers = [
  'bg-[linear-gradient(135deg,#7b74f2,var(--red))]',
  'bg-[linear-gradient(135deg,#1cc0bf,var(--teal))]',
  'bg-[linear-gradient(135deg,#9b6cf5,var(--plum))]',
  'bg-[linear-gradient(135deg,#f7bd4e,var(--gold))]',
  'bg-[linear-gradient(135deg,#3a3950,var(--ink))]',
]
const coverClass = computed(() => covers[(props.index ?? 0) % covers.length])

const title = computed(() => getLocalizedText(props.product?.title))
const categoryName = computed(() => getLocalizedText(props.product?.category?.name))
const soldOut = computed(() => isSoldOut(props.product))
const promo = computed(() => hasPromotionPrice(props.product))

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

// 价签徽章：促销 / 批发 / 活动（择一，对齐 classic 优先级）
const priceSignal = computed<{ tone: string; label: string } | null>(() => {
  if (promo.value) return { tone: 'bg-primary/10 text-primary', label: t('products.promotionTag') }
  if (hasWholesalePrices(props.product)) return { tone: 'bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]', label: t('products.wholesaleTag') }
  if (hasPromotionRules(props.product)) return { tone: 'bg-[color:var(--gold-soft)] text-[color:var(--gold-strong)]', label: t('products.promotionBadge') }
  return null
})
</script>
