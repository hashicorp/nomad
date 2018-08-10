import { getOwner } from '@ember/application';
import Service from '@ember/service';
import { moduleFor, test } from 'ember-qunit';
import Pretender from 'pretender';

moduleFor('service:token', 'Unit | Service | Token', {
  beforeEach() {
    const mockSystem = Service.extend({
      activeRegion: 'region-1',
      shouldIncludeRegion: true,
    });

    this.register('service:system', mockSystem);
    this.system = getOwner(this).lookup('service:system');

    this.server = new Pretender(function() {
      this.get('/path', () => [200, {}, null]);
    });
  },
  afterEach() {
    this.server.shutdown();
  },
  subject() {
    return getOwner(this)
      .factoryFor('service:token')
      .create();
  },
});

test('authorizedRequest includes the region param when the system service says to', function(assert) {
  const token = this.subject();

  token.authorizedRequest('/path');
  assert.equal(
    this.server.handledRequests.pop().url,
    `/path?region=${this.system.get('activeRegion')}`,
    'The region param is included when the system service shouldIncludeRegion property is true'
  );

  this.system.set('shouldIncludeRegion', false);

  token.authorizedRequest('/path');
  assert.equal(
    this.server.handledRequests.pop().url,
    '/path',
    'The region param is not included when the system service shouldIncludeRegion property is false'
  );
});

test('authorizedRawRequest bypasses adding the region param', function(assert) {
  const token = this.subject();

  token.authorizedRawRequest('/path');
  assert.equal(
    this.server.handledRequests.pop().url,
    '/path',
    'The region param is ommitted when making a raw request'
  );
});
