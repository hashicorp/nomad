import Ember from 'ember';

const { Controller, inject } = Ember;

export default Controller.extend({
  token: inject.service(),

  actions: {
    setTokenProperty(property, event) {
      this.get('token').set(property, event.currentTarget.value);
    },

    clearTokenProperties() {
      this.get('token').setProperties({
        secret: undefined,
        accessor: undefined,
      });
    },
  },
});
