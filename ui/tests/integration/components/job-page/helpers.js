/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { click, find } from '@ember/test-helpers';

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

export function expectStartRequest(assert, server, job) {
  const expectedURL = jobURL(job);
  // TODO: Tuesday, this is the part of the service test that's failing
  console.log(
    'expectStartRequest',
    expectedURL,
    job,
    server.pretender.handledRequests
  );
  const request = server.pretender.handledRequests
    .filterBy('method', 'POST')
    .find((req) => req.url === expectedURL);

  const requestPayload = JSON.parse(request.requestBody).Job;

  assert.ok(request, 'POST URL was made correctly');
  assert.ok(
    requestPayload.Stop == null,
    'The Stop signal is not sent in the POST request'
  );
}

export async function expectError(assert, title) {
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
