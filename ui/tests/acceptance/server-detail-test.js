import { currentURL } from 'ember-native-dom-helpers';
import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';
import ServerDetail from 'nomad-ui/tests/pages/servers/detail';

let agent;

moduleForAcceptance('Acceptance | server detail', {
  beforeEach() {
    server.createList('agent', 3);
    agent = server.db.agents[0];
    ServerDetail.visit({ name: agent.name });
  },
});

test('visiting /servers/:server_name', function(assert) {
  assert.equal(currentURL(), `/servers/${encodeURIComponent(agent.name)}`);
});

test('the server detail page should list all tags for the server', function(assert) {
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

test('the list of servers from /servers should still be present', function(assert) {
  assert.equal(ServerDetail.servers.length, server.db.agents.length, '# of servers');
});

test('the active server should be denoted in the table', function(assert) {
  const activeServers = ServerDetail.servers.filter(server => server.isActive);

  assert.equal(activeServers.length, 1, 'Only one active server');
  assert.equal(ServerDetail.activeServer.name, agent.name, 'Active server matches current route');
});

test('when the server is not found, an error message is shown, but the URL persists', function(assert) {
  ServerDetail.visit({ name: 'not-a-real-server' });

  andThen(() => {
    assert.equal(currentURL(), '/servers/not-a-real-server', 'The URL persists');
    assert.equal(ServerDetail.error.title, 'Not Found', 'Error message is for 404');
  });
});
