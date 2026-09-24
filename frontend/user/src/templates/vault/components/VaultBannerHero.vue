<template>
  <section v-if="showHeroSection || !bannerLoading" class="mx-auto w-full max-w-[1180px] px-4 pt-7 sm:px-6">
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
          class="absolute inset-0 h-full w-full object-cover opacity-25 grayscale-[35%]"
        />
      </Transition>

      <div class="relative z-[2] grid min-h-[380px] items-end gap-8 p-6 sm:p-8 md:grid-cols-[minmax(0,1fr)_260px] md:p-10">
        <div class="max-w-[820px]">
          <div class="vault-hero-eyebrow">AI MEMBERSHIP ACCESS</div>
          <h1 class="vault-hero-title mt-6">
            {{ heroTitle || '订阅、兑换、履约，一条链完成' }}
          </h1>
          <p class="vault-hero-copy mt-6">
            {{ heroSubtitle || '选择需要的 AI 订阅服务，完成兑换后，订单直接进入履约流程。' }}
          </p>

          <div class="mt-7 flex flex-wrap gap-2">
            <span class="vault-hero-step"><b>01</b> SELECT</span>
            <span class="vault-hero-step"><b>02</b> REDEEM</span>
            <span class="vault-hero-step"><b>03</b> FULFILL</span>
          </div>

          <div class="mt-8 flex flex-wrap items-center gap-3">
            <RouterLink
              to="/products"
              class="inline-flex min-h-[44px] items-center gap-2 rounded-[9px] bg-primary px-5 py-2.5 text-sm font-black text-primary-foreground transition hover:-translate-y-0.5 hover:bg-primary-hover"
            >
              {{ heroPrimaryButtonText || '查看服务' }}
              <ArrowRight class="h-4 w-4" />
            </RouterLink>
            <span class="text-xs font-bold tracking-[0.08em] text-white/45">NO GENERIC STOREFRONT</span>
          </div>
        </div>

        <aside class="hidden self-stretch md:flex md:flex-col md:justify-between md:border-l md:border-white/10 md:pl-7">
          <div class="text-[11px] font-black uppercase tracking-[0.16em] text-white/40">SERVICE DESK / 01</div>
          <div>
            <div class="text-[72px] font-black leading-none tracking-[-0.08em] text-primary">AI</div>
            <div class="mt-2 text-sm font-bold uppercase tracking-[0.12em] text-white/55">Subscription<br />Access</div>
          </div>
          <div class="flex items-center gap-2 text-xs font-bold text-white/55">
            <span class="h-2 w-2 rounded-[2px] bg-primary"></span>
            ONLINE / READY
          </div>
        </aside>
      </div>

      <div v-if="bannerCount > 1" class="absolute bottom-5 right-5 z-[3] flex items-center gap-2">
        <button
          v-for="(_, idx) in banners"
          :key="`vault-banner-dot-${idx}`"
          type="button"
          class="h-2 rounded-[2px] transition-all"
          :class="idx === currentBannerIndex ? 'w-7 bg-primary' : 'w-2 bg-white/30 hover:bg-white/60'"
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
  transition: opacity 360ms ease;
}
.vault-banner-fade-enter-from,
.vault-banner-fade-leave-to {
  opacity: 0;
}
</style>
