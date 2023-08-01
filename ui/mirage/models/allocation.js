import { Model, hasMany } from 'ember-cli-mirage';

export default Model.extend({
  taskStates: hasMany('task-state'),
  taskResources: hasMany('task-resource'),
});
