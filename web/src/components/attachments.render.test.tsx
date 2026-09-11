// @vitest-environment happy-dom
// Headless render check of the attachments thumbnail: stubs fetch, mounts
// <AttachmentThumb /> for an image and a video, asserts the img/video-icon
// and the team-only delete button.
import { afterEach, describe, expect, it } from 'vitest'
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { AttachmentThumb } from './attachments'
import type { Attachment } from '@/lib/types'

let root: Root | null = null

afterEach(() => {
  if (root) act(() => root!.unmount())
  root = null
  document.body.innerHTML = ''
})

describe('AttachmentThumb render', () => {
  it('renders an image preview and a team delete affordance', async () => {
    const a: Attachment = {
      id: 'a1',
      filename: 'shot.png',
      content_type: 'image/png',
      size_bytes: 64,
      created_at: '2026-09-12T00:00:00Z',
    }
    const host = document.createElement('div')
    document.body.appendChild(host)
    root = createRoot(host)
    await act(async () => {
      root!.render(<AttachmentThumb a={a} onDelete={() => {}} />)
    })
    const img = document.querySelector('img')
    expect(img?.getAttribute('src')).toBe('/api/attachments/a1')
    expect(img?.getAttribute('alt')).toBe('shot.png')
    expect(document.querySelector('[aria-label="Delete shot.png"]')).toBeTruthy()
  })

  it('renders a video as an icon + name without an img', async () => {
    const a: Attachment = {
      id: 'a2',
      filename: 'clip.webm',
      content_type: 'video/webm',
      size_bytes: 123,
      created_at: '2026-09-12T00:00:00Z',
    }
    const host = document.createElement('div')
    document.body.appendChild(host)
    root = createRoot(host)
    await act(async () => {
      root!.render(<AttachmentThumb a={a} />) // no onDelete → client context
    })
    expect(document.querySelector('img')).toBeNull()
    expect(document.body.textContent ?? '').toContain('clip.webm')
    expect(document.querySelector('[aria-label="Delete clip.webm"]')).toBeNull()
  })
})