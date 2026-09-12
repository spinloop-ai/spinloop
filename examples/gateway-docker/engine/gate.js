// The fake engine's gate and stream branch, shared by every gated route.
//
// The key arrives as the ENGINE_API_KEY environment variable: the shim read it
// from the --api-key-file the daemon pointed the engine at, so the value never
// rides on an argument any local user could read. A request presenting a
// different bearer — or none — is refused. With no key supplied the engine is
// ungated, which is right for one reached only over loopback.
//
// A completion request that asks for a stream gets the server-sent-events
// shape; the reply is one canned body, because what is under test is the
// gateway passing a streamed reply through, not an engine tokenising.

var key = env["ENGINE_API_KEY"] || "";

if (key !== "" && (context.request.headers["Authorization"] || "") !== "Bearer " + key) {
  respond()
    .withStatusCode(401)
    .withHeader("Content-Type", "application/json")
    .withContent('{"error":{"message":"incorrect API key provided","type":"invalid_request_error"}}')
    .skipDefaultBehaviour();
} else if (context.request.method === "POST" &&
    /"stream"\s*:\s*true/.test(context.request.body || "")) {
  respond()
    .withStatusCode(200)
    .withHeader("Content-Type", "text/event-stream")
    .withContent(streamReply(context.request.path))
    .skipDefaultBehaviour();
}
// Otherwise the resource's own canned response applies.

function streamReply(path) {
  if (path === "/v1/completions") {
    return "data: " + JSON.stringify({
      id: "cmpl-1", object: "text_completion", created: 1, model: "fake-model",
      choices: [{ index: 0, text: "Hello from the fake engine", finish_reason: "stop" }]
    }) + "\n\ndata: [DONE]\n\n";
  }
  var first = {
    id: "chatcmpl-1", object: "chat.completion.chunk", created: 1, model: "fake-model",
    choices: [{
      index: 0,
      delta: { role: "assistant", content: "Hello from the fake engine" },
      finish_reason: null
    }]
  };
  var last = {
    id: "chatcmpl-1", object: "chat.completion.chunk", created: 1, model: "fake-model",
    choices: [{ index: 0, delta: {}, finish_reason: "stop" }]
  };
  return "data: " + JSON.stringify(first) +
    "\n\ndata: " + JSON.stringify(last) +
    "\n\ndata: [DONE]\n\n";
}
