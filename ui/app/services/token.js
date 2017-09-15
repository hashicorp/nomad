import Ember from 'ember';

const { Service, computed } = Ember;

export default Service.extend({
  accessor: computed({
    get() {
      return window.sessionStorage.nomadTokenAccessor;
    },
    set(key, value) {
      if (value == null) {
        window.sessionStorage.removeItem('nomadTokenAccessor');
      } else {
        window.sessionStorage.nomadTokenAccessor = value;
      }
      return value;
    },
  }),

  secret: computed({
    get() {
      return window.sessionStorage.nomadTokenSecret;
    },
    set(key, value) {
      if (value == null) {
        window.sessionStorage.removeItem('nomadTokenSecret');
      } else {
        window.sessionStorage.nomadTokenSecret = value;
      }

      return value;
    },
  }),
});
