import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Component from '@ember/component';
import { computed } from '@ember/object';
import { lazyClick } from '../helpers/lazy-click';

export default Component.extend({
  // TODO Switch back to the router service once the service behaves more like Route
  // https://github.com/emberjs/ember.js/issues/15801
  // router: inject.service('router'),
  _router: service('-routing'),
  router: alias('_router.router'),

  tagName: 'tr',
  classNames: ['server-agent-row', 'is-interactive'],
  classNameBindings: ['isActive:is-active'],

  agent: null,
  isActive: computed('agent', 'router.currentURL', function() {
    // TODO Switch back to the router service once the service behaves more like Route
    // https://github.com/emberjs/ember.js/issues/15801
    // const targetURL = this.get('router').urlFor('servers.server', this.get('agent'));
    // const currentURL = `${this.get('router.rootURL').slice(0, -1)}${this.get('router.currentURL')}`;

    const router = this.router;
    const targetURL = router.generate('servers.server', this.agent);
    const currentURL = `${router.get('rootURL').slice(0, -1)}${
      router.get('currentURL').split('?')[0]
    }`;

    // Account for potential URI encoding
    return currentURL.replace(/%40/g, '@') === targetURL.replace(/%40/g, '@');
  }),

  click() {
    const transition = () => this.router.transitionTo('servers.server', this.agent);
    lazyClick([transition, event]);
  },
});
