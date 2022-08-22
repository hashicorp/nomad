// @ts-check
import { attr, belongsTo } from '@ember-data/model';
import Model from '@ember-data/model';
import { alias } from '@ember/object/computed';

export default class Service extends Model {
  @attr('string') address;
  // @attr() allocID;
  @belongsTo('allocation') allocation;
  @attr('number') createIndex;
  @attr('string') datacenter;
  // @attr() ID;
  // @attr() jobID;
  @attr('number') modifyIndex;
  @attr('string') namespace;
  // @attr() nodeID;
  @belongsTo('node') node;
  @attr('number') port;
  @attr('string') serviceName;
  @alias('serviceName') name;
  @attr() tags;
}
