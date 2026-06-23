# User-space Applications

This directory records requirements and designs for applications that exercise
Genesis Kernel without becoming kernel owners.

Application documents follow the same discipline as kernel requirements:
requirements describe production semantics, designs describe ownership and
data flow, implementation plans describe staged delivery, and issues record
current gaps. The boundary is different: application requirements must not
create kernel capabilities unless they reduce the need to an approved generic
kernel primitive.

User-space ingress applications may:

- receive external events;
- normalize those events into application-owned envelopes;
- map external threads or users to opaque kernel sessions;
- submit turns through the kernel HTTP surface;
- keep adapter-local retry, token, signature, and inbox state.

User-space ingress applications must not:

- assemble provider context;
- write kernel ledgers, memory truth, tool results, checkpoints, or audit facts;
- make external channel identity a kernel permission authority;
- expose application-specific rich actions as kernel APIs;
- decide whether, where, or how the LLM should reply to an external channel;
- send Feishu, email, WeChat, or other external replies on behalf of the LLM;
- import kernel internals instead of using public kernel transport or syscalls.

External channel actions are outbound capabilities. They belong to skills,
external CLIs, and kernel-governed generic tools such as `shell_exec`, not to
the ingress runtime.

Application issues belong in `docs/operations/application-issues.md`. Kernel
issues remain in `docs/operations/kernel-issues.md`.
