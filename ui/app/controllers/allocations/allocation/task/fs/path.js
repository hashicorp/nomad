import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { filterBy } from '@ember/object/computed';

export default Controller.extend({
  directories: filterBy('ls', 'IsDir'),
  files: filterBy('ls', 'IsDir', false),

  pathComponents: computed('pathWithTaskName', function() {
    return this.pathWithTaskName
      .split('/')
      .reject(s => s === '')
      .reduce(
        (componentsAndPath, component) => {
          componentsAndPath.components.push({
            name: component,
            path: componentsAndPath.path,
          });

          componentsAndPath.path = `${componentsAndPath.path}/${component}`;

          return componentsAndPath;
        },
        {
          components: [],
          path: '/',
        }
      ).components;
  }),
});
