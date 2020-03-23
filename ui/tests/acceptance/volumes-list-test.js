import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';

module('Acceptance | volumes list', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('visiting /csi', async function(assert) {
    // redirects to /csi/volumes
  });

  test('visiting /csi/volumes', async function(assert) {});

  test('/csi/volumes should list the first page of volumes sorted by name', async function(assert) {});

  test('each volume row should contain information about the volume', async function(assert) {});

  test('each volume row should link to the corresponding volume', async function(assert) {});

  test('when there are no volumes, there is an empty message', async function(assert) {});

  test('when the namespace query param is set, only matching volumes are shown and the namespace value is forwarded to app state', async function(assert) {});

  test('when accessing volumes is forbidden, a message is shown with a link to the tokens page', async function(assert) {});
});
