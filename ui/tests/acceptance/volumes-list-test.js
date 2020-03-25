import { module, skip } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';

module('Acceptance | volumes list', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  skip('visiting /csi', async function() {
    // redirects to /csi/volumes
  });

  skip('visiting /csi/volumes', async function() {});

  skip('/csi/volumes should list the first page of volumes sorted by name', async function() {});

  skip('each volume row should contain information about the volume', async function() {});

  skip('each volume row should link to the corresponding volume', async function() {});

  skip('when there are no volumes, there is an empty message', async function() {});

  skip('when the namespace query param is set, only matching volumes are shown and the namespace value is forwarded to app state', async function() {});

  skip('when accessing volumes is forbidden, a message is shown with a link to the tokens page', async function() {});
});
