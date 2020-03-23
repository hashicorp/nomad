// import { run } from '@ember/runloop';
// import { assign } from '@ember/polyfills';
// import { settled } from '@ember/test-helpers';
import { setupTest } from 'ember-qunit';
import { module, test } from 'qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
// import XHRToken from 'nomad-ui/utils/classes/xhr-token';

module('Unit | Adapter | Job', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(async function() {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.adapterFor('job');

    window.sessionStorage.clear();
    window.localStorage.clear();

    this.server = startMirage();

    this.initializeUI = async () => {
      this.server.create('namespace');
      this.server.create('namespace', { id: 'some-namespace' });
      this.server.create('node');
      this.server.create('job', { id: 'job-1', namespaceId: 'default' });
      this.server.create('job', { id: 'job-2', namespaceId: 'some-namespace' });

      this.server.create('region', { id: 'region-1' });
      this.server.create('region', { id: 'region-2' });

      this.system = this.owner.lookup('service:system');

      // Namespace, default region, and all regions are requests that all
      // job requests depend on. Fetching them ahead of time means testing
      // job adapter behavior in isolation.
      await this.system.get('namespaces');
      this.system.get('shouldIncludeRegion');
      await this.system.get('defaultRegion');

      // Reset the handledRequests array to avoid accounting for this
      // namespaces request everywhere.
      this.server.pretender.handledRequests.length = 0;
    };
  });

  hooks.afterEach(function() {
    this.server.shudown();
  });

  test('The volume endpoint can be queried by type', async function(assert) {});
  test('When a namespace is set in localStorage but a volume in the default namespace is requested, the namespace query param is not present', async function(assert) {});
  test('When the volume has a namespace other than default, it is in the URL', async function(assert) {});
  test('When a namespace is set in localStorage and volumes are queries, the namespace is in the URL', async function(assert) {});
  test('query can be watched', async function(assert) {});
  test('query can be canceled', async function(assert) {});
  test('query and findAll have distinct watchList entries', async function(assert) {});
});
