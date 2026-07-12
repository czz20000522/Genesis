import assert from 'node:assert/strict'

import { activeProfileForRole, isLocalProfile, profileDisplayName } from '../src/modelSelection.ts'

const profiles = [
  { profile_id: 'local-qwen', model_id: 'Qwen AgentWorld', gateway_route: 'local-llama-cpp', provider_adapter_id: 'llama.cpp' },
  { profile_id: 'cloud-deepseek-flash', model_id: 'deepseek-v4-flash', gateway_route: 'deepseek', provider_adapter_id: 'deepseek' },
]

assert.equal(activeProfileForRole(profiles, {}, 'coordinator'), undefined)
assert.equal(activeProfileForRole(profiles, { coordinator: 'cloud-deepseek-flash' }, 'coordinator')?.profile_id, 'cloud-deepseek-flash')
assert.equal(activeProfileForRole(profiles, { coordinator: 'missing' }, 'coordinator'), undefined)

assert.equal(isLocalProfile(profiles[0]), true)
assert.equal(isLocalProfile(profiles[1]), false)
assert.equal(profileDisplayName(undefined), '选择模型')
assert.equal(profileDisplayName(profiles[1]), 'deepseek-v4-flash')
