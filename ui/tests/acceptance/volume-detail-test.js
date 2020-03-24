import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';

module('Acceptance | volume detail', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(function() {});

  test('/csi/volums/:id should have a breadcrumb trail linking back to Volumes and CSI', async function(assert) {});

  test('/csi/volumes/:id should show the volume name in the title', async function(assert) {});

  test('/csi/volumes/:id should list additional details for the volume below the title', async function(assert) {});

  test('/csi/volumes/:id should list all write allocations the volume is attached to', async function(assert) {});

  test('/csi/volumes/:id should list all read allocations the volume is attached to', async function(assert) {});

  test('each allocation should have high-level details forthe allocation', async function(assert) {});

  test('each allocation should link to the allocation detail page', async function(assert) {});

  test('when there are no write allocations, the table presents an empty state', async function(assert) {});

  test('when there are no read allocations, the table presents an empty state', async function(assert) {});
});
