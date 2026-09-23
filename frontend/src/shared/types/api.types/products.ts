import type { Body, Ok, Spec } from './spec';

// ─── Products (pos-cashbox T5a) ───

export type Product = Spec<'Product'>;
export type StockMovement = Spec<'StockMovement'>;

export type ProductsListResponse = Ok<'productsList'>;
export type ProductEnvelopeResponse = Ok<'productsGet'>;
// `productsCreate` and `productsUpdate` answer the same `{ product: Product }`
// shape as `productsGet` — one name (`ProductEnvelopeResponse`) instead of
// three structurally identical types, same reasoning as `CashSessionOpenResponse`.
export type ProductStockMovementsResponse = Ok<'productsListStockMovements'>;
// `productsRestock` and `productsAdjust` both answer `{ product, stock_movement }`.
export type ProductStockWriteResponse = Ok<'productsRestock'>;

export type CreateProductRequest = Body<'productsCreate'>;
export type UpdateProductRequest = Body<'productsUpdate'>;
export type RestockProductRequest = Body<'productsRestock'>;
export type AdjustProductRequest = Body<'productsAdjust'>;
