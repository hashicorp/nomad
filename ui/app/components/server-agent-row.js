import Ember from 'ember';
import { lazyClick } from '../helpers/lazy-click';

const { Component, inject, computed } = Ember;

export default Component.extend({
  // TODO Switch back to the router service style when it is no longer feature-flagged
  // router: inject.service('router'),
  _router: inject.service('-routing'),
  router: computed.alias('_router.router'),

  tagName: 'tr',
  classNames: ['server-agent-row', 'is-interactive'],
  classNameBindings: ['isActive:is-active'],

  agent: null,
  isActive: computed('agent', 'router.currentURL', function() {
    // TODO Switch back to the router service style when it is no longer feature-flagged
    // const targetURL = this.get('router').urlFor('servers.server', this.get('agent'));
    // const currentURL = `${this.get('router.rootURL').slice(0, -1)}${this.get('router.currentURL')}`;

    const router = this.get('router');
    const targetURL = router.generate('servers.server', this.get('agent'));
    const currentURL = `${router.get('rootURL').slice(0, -1)}${router
      .get('currentURL')
      .split('?')[0]}`;

    return currentURL === targetURL;
  }),

  click() {
    const transition = () => this.get('router').transitionTo('servers.server', this.get('agent'));
    lazyClick([transition, event]);
  },
});
