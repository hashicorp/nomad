import Mixin from '@ember/object/mixin';

export default Mixin.create({
  setupController(controller) {
    if (this.get('isForbidden')) {
      this.set('isForbidden', undefined);
      controller.set('isForbidden', true);
    }
    this._super(...arguments);
  },

  resetController(controller) {
    controller.set('isForbidden', false);
    this._super(...arguments);
  },
});
