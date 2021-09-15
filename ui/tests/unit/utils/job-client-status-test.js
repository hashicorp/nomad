import { module, test } from 'qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

module('Unit | Util | jobclientstatus', function(hooks) {
  hooks.beforeEach(async function() {
    console.log('hi');
    this.server = startMirage();
  });
  hooks.afterEach(async function() {
    this.server.shutdown();
  });
  test('some test', async function(assert) {
    // this.pauseTest();
    // jobClientStatus('node', 'job');
    assert.equal(0, 0);
  });
});
