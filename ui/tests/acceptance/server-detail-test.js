import { test } from 'qunit';
import moduleForAcceptance from 'nomad-ui/tests/helpers/module-for-acceptance';

let agent;

moduleForAcceptance('Acceptance | server detail', {
  beforeEach() {
    server.createList('agent', 3);
    agent = server.db.agents[0];
    visit(`/servers/${agent.name}`);
  },
});

test('visiting /servers/:server_name', function(assert) {
  assert.equal(currentURL(), `/servers/${agent.name}`);
});

test('the server detail page should list all tags for the server', function(assert) {
  const tags = agent.tags;

  assert.equal(find('.server-tags tbody tr').length, Object.keys(tags).length, '# of tags');
  Object.keys(tags).forEach((key, index) => {
    const row = find(`.server-tags tbody tr:eq(${index})`);
    assert.equal(row.find('td:eq(0)').text(), key, `Label: ${key}`);
    assert.equal(row.find('td:eq(1)').text(), tags[key], `Value: ${tags[key]}`);
  });
});

test('the list of servers from /servers should still be present', function(assert) {
  assert.equal(find('.server-agent-row').length, server.db.agents.length, '# of servers');
});

test('the active server should be denoted in the table', function(assert) {
  assert.equal(find('.server-agent-row.is-active').length, 1, 'Only one active server');
  assert.equal(
    find('.server-agent-row.is-active td:eq(0)').text(),
    agent.name,
    'Active server matches current route'
  );
});
