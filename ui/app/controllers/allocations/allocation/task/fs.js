import Controller from '@ember/controller';
import { computed } from '@ember/object';
import { filterBy } from '@ember/object/computed';

export default Controller.extend({
  directories: filterBy('directoryEntries', 'IsDir'),
  files: filterBy('directoryEntries', 'IsDir', false),

  pathComponents: computed('path', 'model.name', function() {
    return this.path
      .split('/')
      .reject(s => s === '')
      .reduce(
        (componentsAndPath, component, componentIndex, components) => {
          if (componentIndex) {
            componentsAndPath.path = `${componentsAndPath.path}/${component}`;
          } else {
            componentsAndPath.path = component;
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
          path: '',
        }
      ).components;
  }),
});
