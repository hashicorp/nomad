import { currentURL } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import ServerDetail from 'nomad-ui/tests/pages/servers/detail';

let agent;

module('Acceptance | server detail', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  hooks.beforeEach(async function() {
    server.createList('agent', 3);
    agent = server.db.agents[0];
    await ServerDetail.visit({ name: agent.name });
  });

  test('visiting /servers/:server_name', async function(assert) {
    assert.equal(currentURL(), `/servers/${encodeURIComponent(agent.name)}`);
    assert.equal(document.title, `Server ${agent.name} - Nomad`);
  });

  test('the server detail page should list all tags for the server', async function(assert) {
    const tags = Object.keys(agent.tags)
      .map(name => ({ name, value: agent.tags[name] }))
      .sortBy('name');

    assert.equal(ServerDetail.tags.length, tags.length, '# of tags');
    ServerDetail.tags.forEach((tagRow, index) => {
      const tag = tags[index];
      assert.equal(tagRow.name, tag.name, `Label: ${tag.name}`);
      assert.equal(tagRow.value, tag.value, `Value: ${tag.value}`);
    });
  });

  test('when the server is not found, an error message is shown, but the URL persists', async function(assert) {
    await ServerDetail.visit({ name: 'not-a-real-server' });

    assert.equal(currentURL(), '/servers/not-a-real-server', 'The URL persists');
    assert.equal(ServerDetail.error.title, 'Not Found', 'Error message is for 404');
  });
});
