import { click, find } from 'ember-native-dom-helpers';
import wait from 'ember-test-helpers/wait';

export function jobURL(job, path = '') {
  const id = job.get('plainId');
  const namespace = job.get('namespace.name') || 'default';
  let expectedURL = `/v1/job/${id}${path}`;
  if (namespace !== 'default') {
    expectedURL += `?namespace=${namespace}`;
  }
  return expectedURL;
}

export function stopJob() {
  click('[data-test-stop] [data-test-idle-button]');
  return wait().then(() => {
    click('[data-test-stop] [data-test-confirm-button]');
    return wait();
  });
}

export function startJob() {
  click('[data-test-start] [data-test-idle-button]');
  return wait().then(() => {
    click('[data-test-start] [data-test-confirm-button]');
    return wait();
  });
}

export function expectStartRequest(assert, server, job) {
  const expectedURL = jobURL(job);
  const request = server.pretender.handledRequests
    .filterBy('method', 'POST')
    .find(req => req.url === expectedURL);

  const requestPayload = JSON.parse(request.requestBody).Job;

  assert.ok(request, 'POST URL was made correctly');
  assert.ok(requestPayload.Stop == null, 'The Stop signal is not sent in the POST request');
}

export function expectError(assert, title) {
  return () => {
    assert.equal(
      find('[data-test-job-error-title]').textContent,
      title,
      'Appropriate error is shown'
    );
    assert.ok(
      find('[data-test-job-error-body]').textContent.includes('ACL'),
      'The error message mentions ACLs'
    );

    click('[data-test-job-error-close]');
    assert.notOk(find('[data-test-job-error-title]'), 'Error message is dismissable');
    return wait();
  };
}

export function expectDeleteRequest(assert, server, job) {
  const expectedURL = jobURL(job);

  assert.ok(
    server.pretender.handledRequests
      .filterBy('method', 'DELETE')
      .find(req => req.url === expectedURL),
    'DELETE URL was made correctly'
  );

  return wait();
}
