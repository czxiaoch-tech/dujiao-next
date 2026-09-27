import { api } from './client'

export const rechargePortalAPI = {
    preview: (code: string) => api.post('/public/recharge/preview', { code }),
    preflight: (redemptionToken: string, sessionJSON: string) =>
        api.post('/public/recharge/preflight', {
            redemption_token: redemptionToken,
            session_json: sessionJSON,
        }),
    redeem: (preflightToken: string) =>
        api.post('/public/recharge/redeem', {
            preflight_token: preflightToken,
        }),
    result: (token: string) =>
        api.get('/public/recharge/result', {
            params: { token },
            silentBusinessError: true,
        }),
}
