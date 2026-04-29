/**
 * gh-proxy Cloudflare Workers 版
 * 支持 GitHub 和 Hugging Face 加速
 */

const ASSET_URL = 'https://hunshcn.github.io/gh-proxy' // 首页来源
const JSDELIVR = false // 是否使用 jsDelivr 加速 GitHub blob/raw

// 规则解析函数
function parseRules(str) {
    if (!str) return []
    return str.split('\n')
        .map(line => line.split('#')[0].trim())
        .filter(line => line !== '')
        .map(line => {
            const parts = line.split('/')
            return { user: parts[0], repo: parts[1] || '' }
        })
}

/**
 * URL 匹配逻辑
 */
function matchURL(u) {
    const patterns = [
        // GitHub
        /^(?:https?:\/\/)?github\.com\/([^/]+?)\/([^/]+?)\/(?:releases|archive)\/.*$/i,
        /^(?:https?:\/\/)?github\.com\/([^/]+?)\/([^/]+?)\/(?:blob|raw)\/.*$/i,
        /^(?:https?:\/\/)?github\.com\/([^/]+?)\/([^/]+?)\/(?:info|git-).*$/i,
        /^(?:https?:\/\/)?raw\.(?:githubusercontent|github)\.com\/([^/]+?)\/([^/]+?)\/.+?\/.+$/i,
        /^(?:https?:\/\/)?gist\.(?:githubusercontent|github)\.com\/([^/]+?)\/.+?\/.+$/i,
        // Hugging Face
        /^(?:https?:\/\/)?huggingface\.co\/(datasets\/[^/]+?)\/([^/]+?)\/(?:info|git-|resolve|raw|blob)\/.*$/i,
        /^(?:https?:\/\/)?huggingface\.co\/(datasets\/[^/]+?)\/(?:info|git-|resolve|raw|blob)\/.*$/i,
        /^(?:https?:\/\/)?huggingface\.co\/(spaces\/[^/]+?)\/([^/]+?)\/(?:info|git-|resolve|raw|blob)\/.*$/i,
        /^(?:https?:\/\/)?huggingface\.co\/([^/]+?)\/([^/]+?)\/(?:info|git-|resolve|raw|blob)\/.*$/i,
        /^(?:https?:\/\/)?huggingface\.co\/([^/]+?)\/(?:info|git-|resolve|raw|blob)\/.*$/i,
    ]

    for (const exp of patterns) {
        const m = u.match(exp)
        if (m) return m.slice(1)
    }
    return null
}

function checkACL(rules, groups) {
    if (rules.length === 0) return false
    return rules.some(r => {
        if (r.user === '*') {
            return r.repo !== '' && groups.length >= 2 && groups[1] === r.repo
        }
        if (groups[0] !== r.user) return false
        if (r.repo === '') return true
        return groups.length >= 2 && groups[1] === r.repo
    })
}

export default {
    async fetch(request, env, ctx) {
        const url = new URL(request.url)
        let path = url.pathname.slice(1)
        if (url.search) path += url.search

        // 基础路由
        if (path === "") {
            const q = url.searchParams.get('q')
            if (q) return Response.redirect(url.origin + "/" + q, 302)
            return fetch(ASSET_URL)
        }
        if (path === "favicon.ico") return fetch(ASSET_URL + "/favicon.ico")
        if (path === "healthz") return new Response("ok")

        // 规范化 URL
        let target = path
        if (!target.startsWith('http')) target = 'https://' + target
        target = target.replace(/^https?:\/([^\/])/, 'https://$1') // 修正单斜杠

        const groups = matchURL(target)
        if (!groups) return new Response("无效的输入", { status: 403 })

        // ACL 检查 (从环境变量读取)
        const whiteList = parseRules(env.WHITE_LIST)
        const blackList = parseRules(env.BLACK_LIST)
        const passList = parseRules(env.PASS_LIST)

        if (whiteList.length > 0 && !checkACL(whiteList, groups)) {
            return new Response("白名单限制访问", { status: 403 })
        }
        if (checkACL(blackList, groups)) {
            return new Response("黑名单禁止访问", { status: 403 })
        }

        const isHF = target.includes('huggingface.co')
        const isPass = checkACL(passList, groups)

        // jsDelivr 跳转 (仅针对 GitHub)
        if ((JSDELIVR || isPass) && !isHF) {
            if (target.includes('/blob/') || target.includes('/raw/')) {
                let jd = target.replace('/blob/', '@').replace('/raw/', '@')
                    .replace(/^(https?:\/\/)?github\.com/, 'https://cdn.jsdelivr.net/gh')
                    .replace(/^(https?:\/\/)?raw\.githubusercontent\.com/, 'https://cdn.jsdelivr.net/gh')
                return Response.redirect(jd, 302)
            }
        }

        // 自动将 blob 转换为 raw
        if (target.includes('/blob/')) {
            target = target.replace('/blob/', '/raw/')
        }

        if (isPass) return Response.redirect(target, 302)

        // 代理请求
        const newHdrs = new Headers(request.headers)
        newHdrs.delete('Host')

        const response = await fetch(target, {
            method: request.method,
            headers: newHdrs,
            body: request.body,
            redirect: 'follow'
        })

        const resHdrs = new Headers(response.headers)
        resHdrs.set('Access-Control-Allow-Origin', '*')

        return new Response(response.body, {
            status: response.status,
            statusText: response.statusText,
            headers: resHdrs
        })
    }
}
