/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { click, find, waitFor, waitUntil } from '@ember/test-helpers';

export function jobURL(job, path = '') {
  const id = job.get('plainId');
  const namespace = job.get('namespace.name') || 'default';
  let expectedURL = `/v1/job/${id}${path}`;
  if (namespace !== 'default') {
    expectedURL += `?namespace=${namespace}`;
  }
  return expectedURL;
}

export async function stopJob() {
  await click('[data-test-stop] [data-test-idle-button]');
  await click('[data-test-stop] [data-test-confirm-button]');
}

export async function startJob() {
  await click('[data-test-start] [data-test-idle-button]');
  await click('[data-test-start] [data-test-confirm-button]');
}

export async function purgeJob() {
  await click('[data-test-purge] [data-test-idle-button]');
  await click('[data-test-purge] [data-test-confirm-button]');
}

export async function expectStartRequest(assert, server, job) {
  const expectedUpdateURL = jobURL(job);
  const namespace = job.get('namespace.name') || 'default';
  const expectedRunURL =
    namespace === 'default' ? '/v1/jobs' : `/v1/jobs?namespace=${namespace}`;

  await waitUntil(() => {
    return server.pretender.handledRequests
      .filterBy('method', 'POST')
      .some((req) => isStartRequest(req, expectedUpdateURL, expectedRunURL));
  });

  const request = server.pretender.handledRequests
    .filterBy('method', 'POST')
    .find((req) => isStartRequest(req, expectedUpdateURL, expectedRunURL));

  assert.ok(request, 'POST URL was made correctly');
}

export async function expectError(assert, title) {
  await waitFor('[data-test-job-error-title]');

  assert.equal(
    find('[data-test-job-error-title]').textContent,
    title,
    'Appropriate error is shown'
  );
  assert.ok(
    find('[data-test-job-error-body]').textContent.includes('ACL'),
    'The error message mentions ACLs'
  );

  await click('[data-test-job-error-close]');
  await waitUntil(() => !find('[data-test-job-error-title]'));

  assert.notOk(
    find('[data-test-job-error-title]'),
    'Error message is dismissable'
  );
}

export function expectDeleteRequest(assert, server, job) {
  const expectedURL = jobURL(job);

  assert.ok(
    server.pretender.handledRequests
      .filterBy('method', 'DELETE')
      .find((req) => req.url === expectedURL),
    'DELETE URL was made correctly'
  );
}

export function expectPurgeRequest(assert, server, job) {
  const expectedURL = jobURL(job) + '?purge=true';

  assert.ok(
    server.pretender.handledRequests
      .filterBy('method', 'DELETE')
      .find((req) => req.url === expectedURL),
    'DELETE URL was made correctly'
  );
}

function normalizeRequestURL(url = '') {
  if (url.startsWith('/')) {
    return url;
  }

  if (url.startsWith('//')) {
    const parsed = new URL(`http:${url}`);
    return `${parsed.pathname}${parsed.search}`;
  }

  try {
    const parsed = new URL(url);
    return `${parsed.pathname}${parsed.search}`;
  } catch {
    return url;
  }
}

function isStartRequest(request, expectedUpdateURL, expectedRunURL) {
  const url = normalizeRequestURL(request.url);

  if (url === expectedUpdateURL || url === expectedRunURL) {
    return true;
  }

  const updateQuerySeparator = expectedUpdateURL.includes('?') ? '&' : '?';
  const runQuerySeparator = expectedRunURL.includes('?') ? '&' : '?';
  const isUpdateRequest = url.startsWith(
    `${expectedUpdateURL}${updateQuerySeparator}`,
  );
  const isRunRequest =
    url.startsWith(`${expectedRunURL}${runQuerySeparator}`) &&
    !url.startsWith('/v1/jobs/parse');

  if (isUpdateRequest || isRunRequest) {
    return true;
  }

  const body = parseRequestBody(request.requestBody);
  return Boolean(body?.Job && body?.Submission);
}

function parseRequestBody(body) {
  if (!body || typeof body !== 'string') {
    return null;
  }

  try {
    return JSON.parse(body);
  } catch {
    return null;
  }
}
