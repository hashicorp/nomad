import defaultMdxComponents from '@hashicorp/nextjs-scripts/lib/providers/docs'

export default function generateComponents(
  productName,
  additionalComponents = {}
) {
  return defaultMdxComponents({
    product: productName,
    additionalComponents,
  })
}
