import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { filterBy } from '@ember/object/computed';

export default Controller.extend({
  directories: filterBy('directoryEntries', 'IsDir'),
  files: filterBy('directoryEntries', 'IsDir', false),

  pathComponents: computed('pathWithTaskName', function() {
    return this.pathWithTaskName
      .split('/')
      .reject(s => s === '')
      .reduce(
        (componentsAndPath, component, componentIndex, components) => {
          if (componentIndex) {
            componentsAndPath.path = `${componentsAndPath.path}/${component}`;
          }

          componentsAndPath.components.push({
            name: component,
            path: componentsAndPath.path,
            isLast: componentIndex === components.length - 1,
          });

          return componentsAndPath;
        },
        {
          components: [],
          path: '/',
        }
      ).components;
  }),
});
