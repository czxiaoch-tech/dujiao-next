import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { processHtmlForDisplay } from '../utils/content'
import { usePageSeo } from './usePageSeo'

const DEFAULT_TERMS_ZH_CN = `
<h2>服务范围</h2>
<p>本站提供第三方数字订阅相关的信息展示、产品码兑换与人工履约服务。当前公开销售商品以商品页实际显示为准。</p>
<h2>购买与交付</h2>
<ol>
<li>用户通过本站指定的第三方交易平台完成付款。</li>
<li>付款成功后，交易平台自动发放产品码。</li>
<li>用户登录本站，在个人中心兑换产品码并提交准确的充值账号信息。</li>
<li>提交成功后订单进入人工履约流程，用户可在个人中心查看处理状态。</li>
</ol>
<h2>第三方平台与品牌声明</h2>
<p>本站为独立第三方数字服务站，并非 OpenAI 官方网站，也不代表 OpenAI。ChatGPT、OpenAI 及相关名称、商标归其权利人所有。第三方平台的服务状态、规则与价格可能发生变化。</p>
<h2>用户责任</h2>
<p>用户应确保提交的账号、邮箱及其他履约信息真实、准确且有权使用，并遵守相关第三方平台的服务条款。因信息填写错误、账号状态异常或违反第三方规则导致的延迟或失败，需结合实际情况处理。</p>
<h2>退款与售后</h2>
<p>数字商品具有即时交付与可复制特性。产品码尚未使用、订单尚未进入履约前，可联系客服说明情况；产品码已使用或履约已开始后，是否可退款需根据订单实际状态、第三方服务可恢复性及已发生的成本判断。若因本站原因无法完成履约，将按实际订单情况处理退款或补救。</p>
<h2>风险与服务变化</h2>
<p>第三方服务可能因政策、风控、区域、账号状态或技术原因发生变化。本站会尽力完成已接受的订单，但不承诺第三方服务永久可用或规则永久不变。</p>
<h2>联系我们</h2>
<p>如对订单、产品码或履约状态有疑问，可通过本站公开的联系入口提交问题。</p>
`

const DEFAULT_PRIVACY_ZH_CN = `
<h2>我们收集的信息</h2>
<p>为完成注册、订单查询与人工履约，本站可能处理邮箱、订单信息、产品码兑换记录、用户主动提交的充值账号信息，以及必要的安全与访问日志。</p>
<h2>信息使用目的</h2>
<p>上述信息仅用于账号管理、订单处理、产品码兑换、人工履约、售后支持、安全审计与防止滥用。</p>
<h2>支付信息</h2>
<p>付款由第三方交易与支付平台处理。本站不会要求用户向本站提交支付宝支付密码等支付凭据，也不会在本站保存此类支付密码。</p>
<h2>敏感数据保护</h2>
<p>产品兑换码等敏感数据采用受控存储与脱敏展示策略；普通页面与常规日志不应显示完整产品码。只有在确有业务需要的授权路径中才进行必要处理。</p>
<h2>第三方服务</h2>
<p>为完成支付、订单与数字服务交付，部分信息可能由交易平台、支付服务商或实际履约所依赖的第三方服务处理。其数据处理同时受对应第三方规则约束。</p>
<h2>保存与安全</h2>
<p>我们按照完成交易、售后、安全审计及合理合规需要保存必要数据，并采取访问控制、最小暴露和技术保护措施降低数据泄露风险。</p>
<h2>用户权利</h2>
<p>用户可通过本站公开联系入口咨询其订单与账户相关信息；如需更正明显错误的履约信息或提出合理的数据处理请求，可联系客服处理。</p>
<h2>更新</h2>
<p>如服务流程或数据处理方式发生实质变化，本隐私政策可能同步更新，以页面最新版本为准。</p>
`

/**
 * 条款/隐私页共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/Legal.vue 的行为，仅抽离为 composable。
 * type 以 getter 传入以保持响应性（来源于组件 props）。
 */
export function useLegal(type: () => 'terms' | 'privacy') {
  const { t } = useI18n()
  const appStore = useAppStore()

  usePageSeo({
    title: () => type() === 'terms' ? t('footer.terms') : t('footer.privacy'),
    canonicalPath: () => type() === 'terms' ? '/terms' : '/privacy',
  })

  const loading = computed(() => appStore.loading)
  const locale = computed(() => appStore.locale)

  const title = computed(() => {
    return type() === 'terms' ? t('footer.terms') : t('footer.privacy')
  })

  const content = computed(() => {
    const config = appStore.config
    const legal = config?.legal
    const lang = locale.value

    if (type() === 'terms') {
      const configured = legal?.terms?.[lang] || legal?.terms?.['zh-CN'] || ''
      return processHtmlForDisplay(String(configured).trim() || DEFAULT_TERMS_ZH_CN)
    }

    const configured = legal?.privacy?.[lang] || legal?.privacy?.['zh-CN'] || ''
    return processHtmlForDisplay(String(configured).trim() || DEFAULT_PRIVACY_ZH_CN)
  })

  return {
    loading,
    title,
    content,
  }
}
