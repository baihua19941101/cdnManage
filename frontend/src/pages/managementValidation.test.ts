import { describe, expect, it } from 'vitest'

import {
  hasDuplicateProjectBindings,
  validateProjectBindingCounts,
} from './managementValidation'

describe('managementValidation', () => {
  it('validates project binding counts and primary flags', () => {
    expect(
      validateProjectBindingCounts({
        bucketCount: 3,
        cdnCount: 0,
        primaryBucketCount: 1,
        primaryCDNCount: 0,
      }),
    ).toBe('存储桶绑定数量最多为 2 个。')

    expect(
      validateProjectBindingCounts({
        bucketCount: 0,
        cdnCount: 3,
        primaryBucketCount: 0,
        primaryCDNCount: 0,
      }),
    ).toBe('CDN 绑定数量最多为 2 个。')

    expect(
      validateProjectBindingCounts({
        bucketCount: 2,
        cdnCount: 2,
        primaryBucketCount: 0,
        primaryCDNCount: 1,
      }),
    ).toBe('存储桶必须且只能设置一个主绑定。')

    expect(
      validateProjectBindingCounts({
        bucketCount: 0,
        cdnCount: 0,
        primaryBucketCount: 0,
        primaryCDNCount: 0,
      }),
    ).toBeNull()

    expect(
      validateProjectBindingCounts({
        bucketCount: 1,
        cdnCount: 0,
        primaryBucketCount: 1,
        primaryCDNCount: 0,
      }),
    ).toBeNull()

    expect(
      validateProjectBindingCounts({
        bucketCount: 0,
        cdnCount: 1,
        primaryBucketCount: 0,
        primaryCDNCount: 1,
      }),
    ).toBeNull()

    expect(
      validateProjectBindingCounts({
        bucketCount: 2,
        cdnCount: 2,
        primaryBucketCount: 1,
        primaryCDNCount: 2,
      }),
    ).toBe('CDN 必须且只能设置一个主绑定。')
  })

  it('accepts valid bindings and detects duplicate project bindings', () => {
    expect(
      validateProjectBindingCounts({
        bucketCount: 2,
        cdnCount: 2,
        primaryBucketCount: 1,
        primaryCDNCount: 1,
      }),
    ).toBeNull()

    expect(
      hasDuplicateProjectBindings([
        { projectId: 1 },
        { projectId: 2 },
      ]),
    ).toBe(false)
    expect(
      hasDuplicateProjectBindings([
        { projectId: 1 },
        { projectId: 1 },
      ]),
    ).toBe(true)
  })
})
