import Model from '@ember-data/model';
import { attr, belongsTo } from '@ember-data/model';
import { action } from '@ember/object';
// import {
//   fragmentOwner,
//   fragmentArray,
//   fragment,
// } from 'ember-data-model-fragments/attributes';

export default class ActionModel extends Model {
  @belongsTo('job') job;
  @attr() args;
  @attr('string') name;
  @attr('string') command;
  @attr('string', { defaultValue: 'tertiary' }) type; // defaultValue doesnt work??

  @attr('string') messageBuffer;

  @action
  perform(params) {
    return this.store.adapterFor('action').perform(this);
  }

}
