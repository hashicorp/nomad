import Component from '@ember/component';
import { task } from 'ember-concurrency';
import fetch from 'nomad-ui/utils/fetch';
import { getOwner } from '@ember/application';
import { bindKeyboardShortcuts, unbindKeyboardShortcuts } from 'ember-keyboard-shortcuts';

export default Component.extend({
  tagName: '',

  opened: false,

  keyboardShortcuts: {
    '/': {
      action: 'open',
      global: false,
      preventDefault: true,
    },
  },

  didInsertElement() {
    this._super(...arguments);
    bindKeyboardShortcuts(this);
  },

  willDestroyElement() {
    this._super(...arguments);
    unbindKeyboardShortcuts(this);
  },

  actions: {
    open() {
      if (this.select) {
        this.select.actions.open();
      }
    },

    storeSelect(select) {
      if (select) {
        this.select = select;
      }
    },
  },

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

    return Object.keys(json.Matches)
      .filter(key => json.Matches[key])
      .map(key => {
        return {
          groupName: key,
          options: json.Matches[key] || [],
        };
      });
  }),
});
