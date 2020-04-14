import Component from '@ember/component';
import { task } from 'ember-concurrency';
import fetch from 'nomad-ui/utils/fetch';
import { getOwner } from '@ember/application';

export default Component.extend({
  tagName: '',

  search: task(function*(prefix) {
    const applicationAdapter = getOwner(this).lookup('adapter:application');
    const searchUrl = applicationAdapter.urlForFindAll('job').replace('jobs', 'search');
    // FIXME hackery!

    const response = yield fetch(searchUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        Prefix: prefix,
        Context: 'all',
      }),
    });
    const json = yield response.json();

    return Object.keys(json.Matches).reduce((results, key) => {
      return results.concat(json.Matches[key] || []);
    }, []);
  }),
});
