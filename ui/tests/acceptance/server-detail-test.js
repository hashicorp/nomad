import $ from 'jquery';
import { find, findAll, currentURL, visit } from 'ember-native-dom-helpers';
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

  assert.equal(findAll('[data-test-server-tag]').length, Object.keys(tags).length, '# of tags');
  Object.keys(tags)
    .map(name => ({ name, value: tags[name] }))
    .sortBy('name')
    .forEach((tag, index) => {
      const row = $(`.server-tags tbody tr:eq(${index})`);
      assert.equal(row.find('td:eq(0)').text(), tag.name, `Label: ${tag.name}`);
      assert.equal(row.find('td:eq(1)').text(), tag.value, `Value: ${tag.value}`);
    });
});

test('the list of servers from /servers should still be present', function(assert) {
  assert.equal(
    findAll('[data-test-server-agent-row]').length,
    server.db.agents.length,
    '# of servers'
  );
});

test('the active server should be denoted in the table', function(assert) {
  assert.equal(
    findAll('[data-test-server-agent-row].is-active').length,
    1,
    'Only one active server'
  );
  assert.equal(
    find('[data-test-server-agent-row].is-active [data-test-server-name]').textContent.trim(),
    agent.name,
    'Active server matches current route'
  );
});

test('when the server is not found, an error message is shown, but the URL persists', function(
  assert
) {
  visit('/servers/not-a-real-server');

  andThen(() => {
    assert.equal(currentURL(), '/servers/not-a-real-server', 'The URL persists');
    assert.ok(find('[data-test-error]'), 'Error message is shown');
    assert.equal(
      find('[data-test-error-title]').textContent,
      'Not Found',
      'Error message is for 404'
    );
  });
});
