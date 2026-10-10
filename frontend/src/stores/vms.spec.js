import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

vi.mock('../services/api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

import api from '../services/api'
import { useVMStore } from './vms'

const awsVM = { id: 'i-123', provider: 'aws', name: 'web-1', state: 'running' }
const gcpVM = { id: '456', provider: 'gcp', name: 'gce-1', state: 'RUNNING' }

describe('vms store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('fetches VMs from a plain array response', async () => {
    api.get.mockResolvedValueOnce({ data: [awsVM, gcpVM] })

    const store = useVMStore()
    await store.fetchVMs()

    expect(store.vms).toHaveLength(2)
    expect(store.warnings).toBe('')
    expect(store.error).toBeNull()
    expect(store.loading).toBe(false)
    expect(api.get).toHaveBeenCalledWith('/api/vms')
  })

  it('fetches VMs and warnings from a structured response', async () => {
    api.get.mockResolvedValueOnce({
      data: {
        vms: [gcpVM],
        warnings: 'aws: failed to describe AWS instances: operation error EC2',
      },
    })

    const store = useVMStore()
    await store.fetchVMs()

    expect(store.vms).toHaveLength(1)
    expect(store.vms[0].provider).toBe('gcp')
    expect(store.warnings).toContain('aws:')
    expect(store.error).toBeNull()
  })

  it('handles an empty structured response', async () => {
    api.get.mockResolvedValueOnce({ data: { vms: [], warnings: '' } })

    const store = useVMStore()
    await store.fetchVMs()

    expect(store.vms).toEqual([])
    expect(store.warnings).toBe('')
  })

  it('falls back to an empty list for an unexpected response shape', async () => {
    api.get.mockResolvedValueOnce({ data: { unexpected: true } })

    const store = useVMStore()
    await store.fetchVMs()

    expect(store.vms).toEqual([])
    expect(store.error).toBeNull()
  })

  it('handles a null vms field in the response', async () => {
    api.get.mockResolvedValueOnce({ data: { vms: null, warnings: '' } })

    const store = useVMStore()
    await store.fetchVMs()

    expect(store.vms).toEqual([])
  })

  it('sets an error and clears VMs on failure', async () => {
    api.get.mockRejectedValueOnce({ response: { data: { error: 'boom' } } })

    const store = useVMStore()
    store.vms = [awsVM]

    await store.fetchVMs()

    expect(store.error).toBe('boom')
    expect(store.vms).toEqual([])
    expect(store.loading).toBe(false)
  })

  it('uses a default error message when the API provides none', async () => {
    api.get.mockRejectedValueOnce({})

    const store = useVMStore()
    await store.fetchVMs()

    expect(store.error).toBe('Failed to fetch VMs')
  })

  it('clears stale warnings on each fetch', async () => {
    const store = useVMStore()

    // First fetch produces a warning.
    api.get.mockResolvedValueOnce({
      data: { vms: [gcpVM], warnings: 'aws: temporarily unavailable' },
    })
    await store.fetchVMs()
    expect(store.warnings).toBe('aws: temporarily unavailable')

    // Second fetch is clean; the stale warning must be gone.
    api.get.mockResolvedValueOnce({ data: { vms: [awsVM, gcpVM], warnings: '' } })
    await store.fetchVMs()
    expect(store.warnings).toBe('')
    expect(store.vms).toHaveLength(2)
  })
})
