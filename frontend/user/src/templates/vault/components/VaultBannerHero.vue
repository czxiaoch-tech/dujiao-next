<template>
  <section v-if="showHeroSection || !bannerLoading" class="mx-auto w-full max-w-[1180px] px-4 pt-5 sm:px-6">
    <div
      class="vault-home-hero"
      @touchstart="onBannerTouchStart"
      @touchend="onBannerTouchEnd"
    >
      <Transition name="vault-banner-fade" mode="out-in">
        <img
          v-if="!bannerLoading && heroImage"
          :src="heroImage"
          :key="heroImage"
          :alt="heroTitle"
          class="absolute inset-0 h-full w-full object-cover opacity-[0.12] grayscale-[40%]"
        />
      </Transition>

      <div class="relative z-[2] grid min-h-[218px] items-center gap-5 p-5 sm:p-6 md:grid-cols-[minmax(0,1fr)_auto] md:px-8 md:py-7">
        <div class="min-w-0 max-w-[760px]">
          <div class="vault-hero-eyebrow">人工智能订阅服务</div>
          <h1 class="vault-hero-title mt-3">
            AI订阅服务
          </h1>
          <p class="vault-hero-copy mt-3">
            选择商品，完成兑换，进入履约。
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-2 md:max-w-[330px] md:justify-end">
          <span class="vault-hero-step"><b>01</b> 选择商品</span>
          <span class="vault-hero-step"><b>02</b> 兑换产品码</span>
          <span class="vault-hero-step"><b>03</b> 等待履约</span>
          <RouterLink
            to="/products"
            class="vault-hero-buy"
          >
            查看商品
            <ArrowRight class="h-4 w-4" />
          </RouterLink>
        </div>
      </div>

      <div v-if="bannerCount > 1" class="absolute bottom-3 right-4 z-[3] flex items-center gap-1.5">
        <button
          v-for="(_, idx) in banners"
          :key="`vault-banner-dot-${idx}`"
          type="button"
          class="h-1.5 rounded-[2px] transition-all"
          :class="idx === currentBannerIndex ? 'w-6 bg-primary' : 'w-1.5 bg-white/30 hover:bg-white/60'"
          :aria-label="t('common.switchBanner', { n: idx + 1 })"
          @click="selectHeroBanner(idx)"
        ></button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight } from 'lucide-vue-next'
import { useBannerCarousel } from '../../../composables/useBannerCarousel'

const { t } = useI18n()

const {
  banners,
  bannerLoading,
  currentBannerIndex,
  bannerCount,
  showHeroSection,
  heroImage,
  heroTitle,
  heroSubtitle,
  heroPrimaryButtonText,
  loadBanners,
  selectHeroBanner,
  onBannerTouchStart,
  onBannerTouchEnd,
  stopHeroAutoPlay,
} = useBannerCarousel()

onMounted(() => { void loadBanners() })
onUnmounted(() => stopHeroAutoPlay())
</script>

<style scoped>
.vault-banner-fade-enter-active,
.vault-banner-fade-leave-active {
  transition: opacity 300ms ease;
}
.vault-banner-fade-enter-from,
.vault-banner-fade-leave-to {
  opacity: 0;
}
</style>
