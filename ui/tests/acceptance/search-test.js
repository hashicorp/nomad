import { module, test } from 'qunit';
import { visit } from '@ember/test-helpers';
import { setupApplicationTest } from 'ember-qunit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import PageLayout from 'nomad-ui/tests/pages/layout';
import { selectSearch } from 'ember-power-select/test-support';
import Response from 'ember-cli-mirage/response';

module('Acceptance | search', function(hooks) {
  setupApplicationTest(hooks);
  setupMirage(hooks);

  test('search searches', async function(assert) {
    assert.timeout(100000);
    await visit('/');

    server.post('/search', function() {
      return new Response(
        200,
        {},
        JSON.stringify({
          Matches: {
            allocs: null,
            deployment: null,
            evals: ['abc2fdc0-e1fd-2536-67d8-43af8ca798ac'],
            jobs: ['abcde'],
            nodes: null,
          },
          Truncations: {
            allocs: 'false',
            deployment: 'false',
            evals: 'false',
            jobs: 'false',
            nodes: 'false',
          },
        })
      );
    });

    await selectSearch(PageLayout.navbar.search.scope, 'abc');

    PageLayout.navbar.search.as(search => {
      assert.equal(search.options.length, 2);
      assert.equal(search.options[0].text, 'abc2fdc0-e1fd-2536-67d8-43af8ca798ac');
      assert.equal(search.options[1].text, 'abcde');
    });
  });
});
