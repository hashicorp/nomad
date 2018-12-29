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

export function expectStopError(assert) {
  return () => {
    assert.equal(
      find('[data-test-job-error-title]').textContent,
      'Could Not Stop Job',
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
