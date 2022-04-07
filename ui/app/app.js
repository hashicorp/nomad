import Application from '@ember/application';
import Resolver from 'ember-resolver';
import loadInitializers from 'ember-load-initializers';
import config from 'nomad-ui/config/environment';
import { deprecate } from '@ember/debug';

import * as string from '@ember/string';

export default class App extends Application {
  modulePrefix = config.modulePrefix;
  podModulePrefix = config.podModulePrefix;
  Resolver = Resolver;
}

loadInitializers(App, config.modulePrefix);

// HACK: The proper @ember/string module depended on by ember-data overrides the
// smoke and mirrors @ember/string module from ember-source that includes the String
// prototype extensions. Instead of attempting to change the build to only include one
// @ember/string definition, we copy/paste the extensions here since we eventually want
// to move away from string extension methods entirely.
let deprecateEmberStringPrototypeExtension = function (
  name,
  fn,
  message = `String prototype extensions are deprecated. Please import ${name} from '@ember/string' instead.`
) {
  return function () {
    deprecate(message, false, {
      id: 'ember-string.prototype-extensions',
      for: 'ember-source',
      since: {
        enabled: '3.24',
      },
      until: '4.0.0',
      url: 'https://deprecations.emberjs.com/v3.x/#toc_ember-string-prototype_extensions',
    });

    return fn(this, ...arguments);
  };
};

Object.defineProperties(String.prototype, {
  /**
    See [String.w](/ember/release/classes/String/methods/w?anchor=w).
    @method w
    @for @ember/string
    @static
    @private
    @deprecated
  */
  w: {
    configurable: true,
    enumerable: false,
    writeable: true,
    value: deprecateEmberStringPrototypeExtension('w', string.w),
  },

  /**
    See [String.loc](/ember/release/classes/String/methods/loc?anchor=loc).
    @method loc
    @for @ember/string
    @static
    @private
    @deprecated
  */
  loc: {
    configurable: true,
    enumerable: false,
    writeable: true,
    value(...args) {
      return string.loc(this, args);
    },
  },

  /**
    See [String.camelize](/ember/release/classes/String/methods/camelize?anchor=camelize).
    @method camelize
    @for @ember/string
    @static
    @private
    @deprecated
  */
  camelize: {
    configurable: true,
    enumerable: false,
    writeable: true,
    value: deprecateEmberStringPrototypeExtension('camelize', string.camelize),
  },

  /**
    See [String.decamelize](/ember/release/classes/String/methods/decamelize?anchor=decamelize).
    @method decamelize
    @for @ember/string
    @static
    @private
    @deprecated
  */
  decamelize: {
    configurable: true,
    enumerable: false,
    writeable: true,
    value: deprecateEmberStringPrototypeExtension(
      'decamelize',
      string.decamelize
    ),
  },

  /**
    See [String.dasherize](/ember/release/classes/String/methods/dasherize?anchor=dasherize).
    @method dasherize
    @for @ember/string
    @static
    @private
    @deprecated
  */
  dasherize: {
    configurable: true,
    enumerable: false,
    writeable: true,
    value: deprecateEmberStringPrototypeExtension(
      'dasherize',
      string.dasherize
    ),
  },

  /**
    See [String.underscore](/ember/release/classes/String/methods/underscore?anchor=underscore).
    @method underscore
    @for @ember/string
    @static
    @private
    @deprecated
  */
  underscore: {
    configurable: true,
    enumerable: false,
    writeable: true,
    value: deprecateEmberStringPrototypeExtension(
      'underscore',
      string.underscore
    ),
  },

  /**
    See [String.classify](/ember/release/classes/String/methods/classify?anchor=classify).
    @method classify
    @for @ember/string
    @static
    @private
    @deprecated
  */
  classify: {
    configurable: true,
    enumerable: false,
    writeable: true,
    value: deprecateEmberStringPrototypeExtension('classify', string.classify),
  },

  /**
    See [String.capitalize](/ember/release/classes/String/methods/capitalize?anchor=capitalize).
    @method capitalize
    @for @ember/string
    @static
    @private
    @deprecated
  */
  capitalize: {
    configurable: true,
    enumerable: false,
    writeable: true,
    value: deprecateEmberStringPrototypeExtension(
      'capitalize',
      string.capitalize
    ),
  },
});
