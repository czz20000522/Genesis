import type { ProviderProfile } from './api/kernelApi'

export function activeProfileForRole(profiles: ProviderProfile[], bindings: Record<string, string>, role: string): ProviderProfile | undefined {
  const profileID = String(bindings[role] ?? '').trim()
  if (!profileID) return undefined
  return profiles.find((profile) => profile.profile_id === profileID)
}

export function isLocalProfile(profile: ProviderProfile | undefined): boolean {
  if (!profile) return false
  return profile.gateway_route === 'local-llama-cpp' || profile.provider_adapter_id === 'llama.cpp'
}

export function profileDisplayName(profile: ProviderProfile | undefined): string {
  return profile?.model_id?.trim() || profile?.profile_id?.trim() || '选择模型'
}
