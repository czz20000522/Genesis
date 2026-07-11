#!/usr/bin/env python3
import argparse
import contextlib
import json
import os
import sys
import urllib.error
import urllib.request

if os.name == "nt":
    import msvcrt
else:
    import fcntl


PROTOCOL = "genesis.provider_command"


def main() -> int:
    parser = argparse.ArgumentParser(description="Genesis provider_command adapter for llama.cpp server")
    parser.add_argument("--base-url", default="http://127.0.0.1:8080/v1")
    parser.add_argument("--timeout-sec", type=float, default=300)
    parser.add_argument("--max-tokens", type=int, default=0)
    parser.add_argument("--lock-path", default="")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return 0

    request = json.load(sys.stdin)
    with single_process_lock(args.lock_path):
        upstream = post_chat_completion(args.base_url, build_chat_payload(request, args.max_tokens), args.timeout_sec)
    json.dump(to_provider_command_response(upstream), sys.stdout, ensure_ascii=False, separators=(",", ":"))
    sys.stdout.write("\n")
    return 0


def build_chat_payload(request, max_tokens=0):
    if request.get("protocol") != PROTOCOL:
        raise SystemExit("unsupported provider_command protocol")
    model = str(request.get("model") or "").strip()
    if not model:
        raise SystemExit("provider_command request missing model")

    conversation = request.get("conversation") or []
    payload = {
        "model": model,
        "messages": conversation_messages(conversation) if conversation else [{"role": "user", "content": model_user_text(request.get("input_items") or [])}],
    }
    tools = chat_tools(request.get("tool_manifest") or [])
    if tools:
        payload["tools"] = tools
        payload["tool_choice"] = "auto"
    if max_tokens > 0:
        payload["max_tokens"] = max_tokens

    if not conversation:
        for round_payload in request.get("tool_rounds") or []:
            calls = chat_tool_calls(round_payload.get("calls") or [])
            if calls:
                payload["messages"].append({"role": "assistant", "tool_calls": calls})
            for result in round_payload.get("results") or []:
                payload["messages"].append({
                    "role": "tool",
                    "tool_call_id": str(result.get("tool_call_id") or "").strip(),
                    "content": str(result.get("content") or ""),
                })
    return payload


def model_user_text(items):
    return "\n".join(str(item.get("text") or "") for item in items if item.get("text"))


def conversation_messages(conversation):
    messages = []
    for message in conversation:
        role = str(message.get("role") or "").strip()
        text = str(message.get("text") or "")
        if role in {"system", "user"}:
            messages.append({"role": role, "content": text})
            continue
        if role == "assistant":
            next_message = {"role": role}
            if text:
                next_message["content"] = text
            calls = chat_tool_calls(message.get("tool_calls") or [])
            if calls:
                next_message["tool_calls"] = calls
            if "content" not in next_message and "tool_calls" not in next_message:
                raise SystemExit("canonical assistant message missing content or tool_calls")
            messages.append(next_message)
            continue
        if role == "tool":
            tool_call_id = str(message.get("tool_call_id") or "").strip()
            if not tool_call_id:
                raise SystemExit("canonical tool message missing tool_call_id")
            messages.append({"role": role, "tool_call_id": tool_call_id, "content": text})
            continue
        raise SystemExit(f"unsupported canonical conversation role: {role}")
    if not messages:
        raise SystemExit("canonical conversation is empty")
    return messages


def chat_tools(tools):
    converted = []
    for tool in tools:
        name = str(tool.get("name") or "").strip()
        if not name:
            continue
        converted.append({
            "type": "function",
            "function": {
                "name": name,
                "description": str(tool.get("description") or "").strip(),
                "parameters": tool.get("input_schema") or {"type": "object"},
            },
        })
    return converted


def chat_tool_calls(calls):
    converted = []
    for call in calls:
        args = call.get("arguments")
        if args is None:
            args = call.get("raw_arguments")
        if args is None:
            args = {}
        converted.append({
            "id": str(call.get("tool_call_id") or "").strip(),
            "type": "function",
            "function": {
                "name": str(call.get("name") or "").strip(),
                "arguments": args if isinstance(args, str) else json.dumps(args, ensure_ascii=False, separators=(",", ":")),
            },
        })
    return converted


def post_chat_completion(base_url, payload, timeout_sec):
    url = base_url.rstrip("/") + "/chat/completions"
    data = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
    request = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"}, method="POST")
    try:
        with urllib.request.urlopen(request, **request_timeout_kwargs(timeout_sec)) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as err:
        body = err.read().decode("utf-8", "replace")[:2000]
        raise SystemExit(f"llama.cpp server returned HTTP {err.code}: {body}") from err
    except urllib.error.URLError as err:
        raise SystemExit(f"llama.cpp server unavailable: {err.reason}") from err


def request_timeout_kwargs(timeout_sec):
    if timeout_sec <= 0:
        return {}
    return {"timeout": timeout_sec}


def to_provider_command_response(upstream):
    choices = upstream.get("choices") or []
    if not choices:
        raise SystemExit("llama.cpp response missing choices")
    message = choices[0].get("message") or {}
    model = str(upstream.get("model") or "").strip()
    usage = token_usage(upstream.get("usage") or {})
    # llama.cpp exposes an OpenAI-compatible reasoning_content field. Convert it
    # into Genesis's semantic provider-command field; the kernel owns persistence.
    # Do not forward a vendor-shaped response or replay directive from this adapter.
    reasoning_text = str(message.get("reasoning_content") or "").strip()
    reasoning = {"text": reasoning_text} if reasoning_text else None

    tool_calls = message.get("tool_calls") or []
    if tool_calls:
        response = {"kind": "tool_calls", "model": model, "tool_calls": provider_tool_calls(tool_calls), "usage": usage}
        if reasoning:
            response["reasoning"] = reasoning
        return response

    text = str(message.get("content") or "")
    if not text.strip():
        raise SystemExit("llama.cpp response missing visible final text")
    response = {"kind": "final", "model": model, "text": text, "usage": usage}
    if reasoning:
        response["reasoning"] = reasoning
    return response


def provider_tool_calls(tool_calls):
    converted = []
    for call in tool_calls:
        function = call.get("function") or {}
        args_text = str(function.get("arguments") or "{}")
        next_call = {
            "tool_call_id": str(call.get("id") or "").strip(),
            "name": str(function.get("name") or "").strip(),
        }
        try:
            next_call["arguments"] = json.loads(args_text)
        except json.JSONDecodeError:
            next_call["raw_arguments"] = args_text
        converted.append(next_call)
    return converted


def token_usage(usage):
    input_tokens = int(usage.get("prompt_tokens") or usage.get("input_tokens") or 0)
    output_tokens = int(usage.get("completion_tokens") or usage.get("output_tokens") or 0)
    total_tokens = int(usage.get("total_tokens") or (input_tokens + output_tokens if input_tokens or output_tokens else 0))
    prompt_details = usage.get("prompt_tokens_details") or {}
    cache_hit = int(usage.get("prompt_cache_hit_tokens") or prompt_details.get("cached_tokens") or 0)
    cache_miss = int(usage.get("prompt_cache_miss_tokens") or (input_tokens - cache_hit if cache_hit and input_tokens > cache_hit else 0))
    out = {
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
        "total_tokens": total_tokens,
        "cache_hit_tokens": cache_hit,
        "cache_miss_tokens": cache_miss,
    }
    return {key: value for key, value in out.items() if value}


@contextlib.contextmanager
def single_process_lock(path):
    if not path:
        yield
        return
    os.makedirs(os.path.dirname(os.path.abspath(path)), exist_ok=True)
    with open(path, "a+b") as handle:
        if handle.tell() == 0 and handle.seek(0, os.SEEK_END) == 0:
            handle.write(b"0")
            handle.flush()
        handle.seek(0)
        # ponytail: single lock matches llama-server --parallel 1; use a route scheduler if local concurrency grows.
        if os.name == "nt":
            msvcrt.locking(handle.fileno(), msvcrt.LK_LOCK, 1)
        else:
            fcntl.flock(handle.fileno(), fcntl.LOCK_EX)
        try:
            yield
        finally:
            handle.seek(0)
            if os.name == "nt":
                msvcrt.locking(handle.fileno(), msvcrt.LK_UNLCK, 1)
            else:
                fcntl.flock(handle.fileno(), fcntl.LOCK_UN)


def self_test():
    request = {
        "protocol": PROTOCOL,
        "model": "local-model",
        "input_items": [{"kind": "user_text", "text": "hello"}],
        "tool_manifest": [{"name": "shell_exec", "description": "run", "input_schema": {"type": "object"}}],
    }
    payload = build_chat_payload(request)
    assert payload["model"] == "local-model"
    assert payload["messages"] == [{"role": "user", "content": "hello"}]
    assert payload["tools"][0]["function"]["name"] == "shell_exec"
    assert "max_tokens" not in payload
    bounded_payload = build_chat_payload(request, 128)
    assert bounded_payload["max_tokens"] == 128
    assert request_timeout_kwargs(0) == {}
    assert request_timeout_kwargs(300) == {"timeout": 300}
    final = to_provider_command_response({
        "model": "local-model",
        "choices": [{"message": {"content": "ok", "reasoning_content": "hidden"}}],
        "usage": {"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5},
    })
    assert final == {
        "kind": "final",
        "model": "local-model",
        "text": "ok",
        "reasoning": {"text": "hidden"},
        "usage": {"input_tokens": 2, "output_tokens": 3, "total_tokens": 5},
    }
    tool = to_provider_command_response({
        "model": "local-model",
        "choices": [{"message": {"reasoning_content": "inspect before tool call", "tool_calls": [{"id": "call_1", "function": {"name": "shell_exec", "arguments": "{\"command\":\"pwd\"}"}}]}}],
    })
    assert tool["tool_calls"][0]["arguments"] == {"command": "pwd"}
    assert tool["reasoning"] == {"text": "inspect before tool call"}
    print("ok")


if __name__ == "__main__":
    raise SystemExit(main())
