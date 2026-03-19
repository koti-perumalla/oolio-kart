import React, { useEffect, useState } from "react";
import { listProducts, placeOrder } from "./api";

export default function OrderPage() {
	const [products, setProducts] = useState([]);
	const [selectedProduct, setSelectedProduct] = useState("");
	const [quantity, setQuantity] = useState(1);
	const [cart, setCart] = useState([]);
	const [couponCode, setCouponCode] = useState("");
	const [message, setMessage] = useState("");
	const [error, setError] = useState("");

	useEffect(() => {
		listProducts().then((data) => {
			setProducts(data);
			if (data.length > 0) {
				setSelectedProduct(data[0].id);
			}
		});
	}, []);

	const selectedProductDetails = products.find((product) => product.id === selectedProduct);

	const addToCart = () => {
		setMessage("");
		setError("");

		if (!selectedProduct) {
			setError("Please select a product.");
			return;
		}

		const parsedQty = Number(quantity);
		if (!parsedQty || parsedQty <= 0) {
			setError("Quantity must be at least 1.");
			return;
		}

		setCart((prev) => {
			const existing = prev.find((item) => item.productId === selectedProduct);
			if (existing) {
				return prev.map((item) =>
					item.productId === selectedProduct
						? { ...item, quantity: item.quantity + parsedQty }
						: item
				);
			}

			return [
				...prev,
				{
					productId: selectedProduct,
					name: selectedProductDetails?.name || selectedProduct,
					price: selectedProductDetails?.price || 0,
					quantity: parsedQty,
				},
			];
		});

		setMessage("Item added to cart.");
	};

	const removeFromCart = (productId) => {
		setCart((prev) => prev.filter((item) => item.productId !== productId));
	};

	const cartTotal = cart.reduce((sum, item) => sum + item.price * item.quantity, 0);

	const submitOrder = async () => {
		setMessage("");
		setError("");

		if (cart.length === 0) {
			setError("Please add at least one item to cart.");
			return;
		}

		const payload = {
			items: cart.map((item) => ({
				productId: item.productId,
				quantity: item.quantity,
			})),
		};

		if (couponCode.trim()) {
			payload.couponCode = couponCode.trim();
		}

		const res = await placeOrder(payload);
		const text = await res.text();

		if (!res.ok) {
			setError(text || "Order failed");
			return;
		}

		const data = JSON.parse(text);
		setMessage(`Order placed successfully. Order ID: ${data.id}`);
		setCart([]);
		setCouponCode("");
	};

	return (
		<div style={{ border: "1px solid #e5e7eb", borderRadius: 10, padding: 16 }}>
			<h2 style={{ marginTop: 0 }}>Place Food Order</h2>

			<div style={{ marginBottom: 16 }}>
				<h3 style={{ marginBottom: 8 }}>Available Products</h3>
				<div style={{ display: "grid", gap: 8 }}>
					{products.map((product) => (
						<label key={product.id} style={{ display: "flex", alignItems: "center", gap: 8 }}>
							<input
								type="radio"
								name="product"
								value={product.id}
								checked={selectedProduct === product.id}
								onChange={() => setSelectedProduct(product.id)}
							/>
							<span>{product.name} - ${product.price}</span>
						</label>
					))}
				</div>
			</div>

			<div style={{ display: "grid", gap: 10, maxWidth: 360, marginBottom: 18 }}>
				<label>
					Quantity
					<input
						type="number"
						min="1"
						value={quantity}
						onChange={(e) => setQuantity(e.target.value)}
						style={{ width: "100%", padding: 8, marginTop: 4 }}
					/>
				</label>

				<button onClick={addToCart}>Add to Cart</button>
			</div>

			<div style={{ borderTop: "1px solid #e5e7eb", paddingTop: 14 }}>
				<h3 style={{ marginTop: 0 }}>Cart</h3>
				{cart.length === 0 ? (
					<p style={{ color: "#6b7280" }}>No items in cart.</p>
				) : (
					<div style={{ display: "grid", gap: 8 }}>
						{cart.map((item) => (
							<div
								key={item.productId}
								style={{
									display: "flex",
									justifyContent: "space-between",
									alignItems: "center",
									border: "1px solid #e5e7eb",
									borderRadius: 8,
									padding: "8px 10px",
								}}
							>
								<div>
									<div>{item.name}</div>
									<small>
										Qty: {item.quantity} | ${item.price} each
									</small>
								</div>
								<button onClick={() => removeFromCart(item.productId)}>Remove</button>
							</div>
						))}
						<div style={{ fontWeight: 700 }}>Cart Total: ${cartTotal.toFixed(2)}</div>
					</div>
				)}
			</div>

			<div style={{ display: "grid", gap: 10, maxWidth: 360, marginTop: 16 }}>
				<label>
					Promo Code (optional)
					<input
						type="text"
						value={couponCode}
						onChange={(e) => setCouponCode(e.target.value)}
						placeholder="8-10 chars"
						style={{ width: "100%", padding: 8, marginTop: 4 }}
					/>
				</label>

				<button onClick={submitOrder}>Checkout & Place Order</button>
			</div>

			{message && <p style={{ color: "#166534", marginTop: 12 }}>{message}</p>}
			{error && <p style={{ color: "#b91c1c", marginTop: 12 }}>{error}</p>}
		</div>
	);
}
