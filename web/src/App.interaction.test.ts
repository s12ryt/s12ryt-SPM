import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import App from './App.vue'
import Trend from './Trend.vue'

const wrappers: VueWrapper[] = []

function reply(value: unknown, status = 200) {
  return { ok: status >= 200 && status < 300, status, json: async () => value }
}

const sessionAdmin = { public: false, admin: true }
const configuration = () => ({
  settings: {
    public: false,
    retentionDays: 7,
    rules: {
      cpu: 90,
      memory: 90,
      disk: 90,
      holdSeconds: 30,
      offlineSeconds: 15,
      intervalSeconds: 3,
    },
    notifications: {
      webhookEnabled: false,
      webhookURL: '',
      telegramEnabled: false,
      telegramToken: '',
      telegramChat: '',
    },
  },
  database: { active: 'sqlite:///spm.db', pending: '', environment: false, error: '' },
})

function buttonByText(w: VueWrapper, text: string) {
  return w.findAll('button').find(b => b.text().includes(text))
}

function formByText(w: VueWrapper, text: string) {
  return w.findAll('form').find(f => f.text().includes(text))
}

async function mountAdmin() {
  vi.stubGlobal('fetch', vi.fn(async (path: string) => {
    if (path.includes('/api/session')) return reply(sessionAdmin)
    return reply([])
  }))
  const w = mount(App)
  wrappers.push(w)
  await flushPromises()
  return w
}

beforeEach(() => {
  vi.useFakeTimers()
})

afterEach(() => {
  wrappers.splice(0).forEach(w => w.unmount())
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('modal interactions', () => {
  it('closes the enrollment modal with Escape', async () => {
    const w = await mountAdmin()
    await buttonByText(w, '新增主機')!.trigger('click')
    await flushPromises()
    expect(w.find('.modal-backdrop').exists()).toBe(true)

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(w.find('.modal-backdrop').exists()).toBe(false)
  })

  it('closes the enrollment modal when clicking the backdrop', async () => {
    const w = await mountAdmin()
    await buttonByText(w, '新增主機')!.trigger('click')
    await flushPromises()
    expect(w.find('.modal-backdrop').exists()).toBe(true)

    await w.find('.modal-backdrop').trigger('click')
    expect(w.find('.modal-backdrop').exists()).toBe(false)
  })
})

describe('success notices', () => {
  it('auto-dismisses a success notice after six seconds', async () => {
    vi.stubGlobal('fetch', vi.fn(async (path: string, init?: RequestInit) => {
      if (path.includes('/api/session')) return reply(sessionAdmin)
      if (path.includes('/api/settings')) return reply(configuration())
      return reply([])
    }))
    const w = mount(App)
    wrappers.push(w)
    await flushPromises()

    await buttonByText(w, '系統設定')!.trigger('click')
    await flushPromises()
    await formByText(w, '儲存設定')!.trigger('submit')
    await flushPromises()
    const banner = w.find('.banner.success')
    expect(banner.exists()).toBe(true)
    expect(banner.text()).toContain('設定已儲存')

    vi.advanceTimersByTime(6000)
    await flushPromises()
    expect(w.find('.banner.success').exists()).toBe(false)
  })

  it('keeps error banners until the next successful refresh', async () => {
    const w = await mountAdmin()
    expect(w.find('.banner.error').exists()).toBe(false)

    vi.stubGlobal('fetch', vi.fn(async () => {
      throw new Error('network down')
    }))
    vi.advanceTimersByTime(3000)
    await flushPromises()
    expect(w.find('.banner.error').exists()).toBe(true)

    vi.advanceTimersByTime(30000)
    await flushPromises()
    expect(w.find('.banner.error').exists()).toBe(true)
  })
})

describe('busy state', () => {
  it('marks the pending primary action with aria-busy', async () => {
    let resolvePut: ((v: unknown) => void) = () => {}
    const put = new Promise<unknown>(resolve => { resolvePut = resolve })
    vi.stubGlobal('fetch', vi.fn(async (path: string, init?: RequestInit) => {
      if (path.includes('/api/session')) return reply(sessionAdmin)
      if (path.includes('/api/settings') && init?.method === 'PUT') return put
      if (path.includes('/api/settings')) return reply(configuration())
      return reply([])
    }))
    const w = mount(App)
    wrappers.push(w)
    await flushPromises()

    await buttonByText(w, '系統設定')!.trigger('click')
    await flushPromises()
    await formByText(w, '儲存設定')!.trigger('submit')
    await flushPromises()
    const save = buttonByText(w, '儲存設定')!
    expect(save.attributes('aria-busy')).toBe('true')

    resolvePut({})
    await flushPromises()
    expect(save.attributes('aria-busy')).toBeUndefined()
  })
})

describe('connecting state', () => {
  it('shows a spinner while the first session request is pending', async () => {
    const pending = new Promise<unknown>(() => {})
    vi.stubGlobal('fetch', vi.fn(async () => pending))
    const w = mount(App)
    wrappers.push(w)
    await flushPromises()
    const empty = w.find('.empty-state')
    expect(empty.exists()).toBe(true)
    expect(empty.find('.spinner').exists()).toBe(true)
  })
})

describe('Trend chart', () => {
  it('renders a closed filled area under the line', () => {
    const w = mount(Trend, {
      props: {
        title: 'CPU',
        unit: '%',
        metric: 'cpu',
        points: [
          { time: 1000, cpu: 40, memory: 40, disk: 40, rx: 0, tx: 0 },
          { time: 2000, cpu: 80, memory: 40, disk: 40, rx: 0, tx: 0 },
        ],
      },
    })
    wrappers.push(w)
    expect(w.find('.trend-line').exists()).toBe(true)
    const area = w.find('.trend-area')
    expect(area.exists()).toBe(true)
    const d = area.attributes('d') ?? ''
    expect(d).toContain('L')
    expect(d.trim().endsWith('Z')).toBe(true)
  })
})
