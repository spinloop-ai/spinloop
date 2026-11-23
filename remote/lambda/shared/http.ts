import type { LambdaFunctionURLEvent, LambdaFunctionURLResult } from 'aws-lambda';

// Names the spinloop version this control plane was deployed with. The CLI
// compares it against its own version on every call, so it rides on every
// response — errors included — rather than costing a separate request.
export const CONTROL_PLANE_VERSION_HEADER = 'x-spinloop-control-plane-version';

export function jsonResponse(
  statusCode: number,
  body: unknown,
  extraHeaders: Record<string, string> = {},
): LambdaFunctionURLResult {
  return {
    statusCode,
    headers: {
      'content-type': 'application/json',
      [CONTROL_PLANE_VERSION_HEADER]: process.env.CONTROL_PLANE_VERSION || 'dev',
      ...extraHeaders,
    },
    body: JSON.stringify(body),
  };
}

/**
 * The invoking principal behind an AWS_IAM Function URL request, for ownership
 * stamps (`spinloop cloud deploy` records who deployed it). A SigV4-signed
 * call carries the resolved caller on the request context — `authorizer.iam`
 * in the payload these URLs deliver, `identity` in other v2 shapes — so read
 * through whichever fields are present, preferring the most human-meaningful:
 * the user/session ARN, then the caller ids, and finally the invoking account
 * (least precise, but an answer). Fall back to `unknown` rather than failing
 * a request over missing metadata. Never anything secret: an identity, nothing
 * more.
 */
export function callerPrincipal(event: LambdaFunctionURLEvent): string {
  const context = event.requestContext as
    | {
        accountId?: string;
        authorizer?: { iam?: Record<string, unknown> };
        identity?: Record<string, unknown>;
      }
    | undefined;
  const iam = context?.authorizer?.iam;
  const identity = context?.identity;
  const candidates: unknown[] = [
    // An IAM user's ARN, or the assumed-role session ARN for a role.
    iam?.userArn,
    // The v2 `identity` shape's names for the same principal.
    identity?.sourceArn,
    identity?.userArn,
    identity?.principalId,
    identity?.username,
    identity?.user,
    // Stable ids and the access key: less readable, still who.
    iam?.callerId,
    iam?.userId,
    iam?.accessKey,
    identity?.accessKey,
    // Least precise, but an answer: the invoking account. `anonymous` is the
    // placeholder an unauthenticated reply carries — no caller at all.
    context?.accountId,
  ];
  for (const value of candidates) {
    if (typeof value === 'string' && value !== '' && value !== 'anonymous') {
      return value;
    }
  }
  return 'unknown';
}
