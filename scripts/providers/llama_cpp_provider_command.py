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
    parser.add_argument("--lock-path", default="")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args()
    if args.self_test:
        self_test()
        return 0

    request = json.load(sys.stdin)
    with single_process_lock(args.lock_path):
        upstream = post_chat_completion(args.base_url, build_chat_payload(request), args.timeout_sec)
    json.dump(to_provider_command_response(upstream), sys.stdout, ensure_ascii=False, separators=(",", ":"))
    sys.stdout.write("\n")
    return 0


def build_chat_payload(request):
    if request.get("protocol") != PROTOCOL:
        raise SystemExit("unsupported provider_command protocol")
    model = str(request.get("model") or "").strip()
    if not model:
        raise SystemExit("provider_command request missing model")

    payload = {
        "model": model,
        "messages": [{"role": "user", "content": model_user_text(request.get("input_items") or [])}],
    }
    tools = chat_tools(request.get("tool_manifest") or [])
    if tools:
        payload["tools"] = tools
        payload["tool_choice"] = "auto"

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
        with urllib.request.urlopen(request, timeout=timeout_sec) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as err:
        body = err.read().decode("utf-8", "replace")[:2000]
        raise SystemExit(f"llama.cpp server returned HTTP {err.code}: {body}") from err
    except urllib.error.URLError as err:
        raise SystemExit(f"llama.cpp server unavailable: {err.reason}") from err


def to_provider_command_response(upstream):
    choices = upstream.get("choices") or []
    if not choices:
        raise SystemExit("llama.cpp response missing choices")
    message = choices[0].get("message") or {}
    model = str(upstream.get("model") or "").strip()
    usage = token_usage(upstream.get("usage") or {})

    tool_calls = message.get("tool_calls") or []
    if tool_calls:
        return {"kind": "tool_calls", "model": model, "tool_calls": provider_tool_calls(tool_calls), "usage": usage}

    text = str(message.get("content") or "")
    if not text.strip():
        raise SystemExit("llama.cpp response missing visible final text")
    return {"kind": "final", "model": model, "text": text, "usage": usage}


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
    final = to_provider_command_response({
        "model": "local-model",
        "choices": [{"message": {"content": "ok", "reasoning_content": "hidden"}}],
        "usage": {"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5},
    })
    assert final == {
        "kind": "final",
        "model": "local-model",
        "text": "ok",
        "usage": {"input_tokens": 2, "output_tokens": 3, "total_tokens": 5},
    }
    tool = to_provider_command_response({
        "model": "local-model",
        "choices": [{"message": {"tool_calls": [{"id": "call_1", "function": {"name": "shell_exec", "arguments": "{\"command\":\"pwd\"}"}}]}}],
    })
    assert tool["tool_calls"][0]["arguments"] == {"command": "pwd"}
    print("ok")


if __name__ == "__main__":
    raise SystemExit(main())
