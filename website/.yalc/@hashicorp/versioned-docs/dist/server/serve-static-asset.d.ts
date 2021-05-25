import type { NextApiRequest, NextApiResponse } from 'next';
export declare function makeServeStaticAssets(product: string): (req: NextApiRequest, res: NextApiResponse) => Promise<void>;
