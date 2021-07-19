module.exports = {
  ...require('@hashicorp/platform-cli/config/.stylelintrc'),
  rules: {
    'selector-pseudo-class-no-unknown': [
      true,
      {
        ignorePseudoClasses: ['first', 'last'],
      },
    ],
  },
}
