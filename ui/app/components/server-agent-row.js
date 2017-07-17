import Ember from 'ember';

const { Component, inject, computed } = Ember;

export default Component.extend({
  router: inject.service(),

  tagName: 'tr',
  classNames: ['server-agent-row', 'is-interactive'],
  classNameBindings: ['isActive:is-active'],

  agent: null,
  isActive: computed('agent', 'router.currentURL', function() {
    const targetURL = this.get('router').urlFor('nodes.servers.server', this.get('agent'));
    const currentURL = `${this.get('router.rootURL').slice(0, -1)}${this.get('router.currentURL')}`;
    return currentURL === targetURL;
  }),

  click() {
    this.get('router').transitionTo('nodes.servers.server', this.get('agent'));
  },
});
