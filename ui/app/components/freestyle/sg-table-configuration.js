import Component from '@ember/component';

export default Component.extend({
  attributes: {
    key: 'val',
    deep: {
      key: 'val',
      more: 'stuff',
    },
    array: ['one', 'two', 'three', 'four'],
    very: {
      deep: {
        key: {
          incoming: {
            one: 1,
            two: 2,
            three: 3,
            four: 'surprisingly long value that is unlike the other properties in this object',
          },
        },
      },
    },
  },
});
