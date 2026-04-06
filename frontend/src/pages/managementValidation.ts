export type ProjectBindingCounts = {
  bucketCount: number
  cdnCount: number
  primaryBucketCount: number
  primaryCDNCount: number
}

export const validateProjectBindingCounts = (
  input: ProjectBindingCounts,
): string | null => {
  if (input.bucketCount < 1 || input.bucketCount > 2) {
    return '存储桶绑定数量必须为 1 到 2 个。'
  }
  if (input.cdnCount < 1 || input.cdnCount > 2) {
    return 'CDN 绑定数量必须为 1 到 2 个。'
  }
  if (input.primaryBucketCount !== 1) {
    return '存储桶必须且只能设置一个主绑定。'
  }
  if (input.primaryCDNCount !== 1) {
    return 'CDN 必须且只能设置一个主绑定。'
  }
  return null
}

export const hasDuplicateProjectBindings = (
  bindings: Array<{ projectId: number }>,
) => {
  const uniqueProjectIDs = new Set(bindings.map((item) => item.projectId))
  return uniqueProjectIDs.size !== bindings.length
}

