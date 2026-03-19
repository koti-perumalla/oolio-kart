import React from "react";
import { BrowserRouter, Navigate, NavLink, Route, Routes } from "react-router-dom";
import OrderPage from "./OrderPage";
import CouponPage from "./CouponPage";

const linkStyle = ({ isActive }) => ({
	padding: "10px 14px",
	borderRadius: 8,
	textDecoration: "none",
	color: isActive ? "#fff" : "#1f2937",
	background: isActive ? "#2563eb" : "#e5e7eb",
	fontWeight: 600,
});

export default function App() {
	return (
		<BrowserRouter>
			<div style={{ maxWidth: 980, margin: "0 auto", padding: 16, fontFamily: "Arial, sans-serif" }}>
				<h1 style={{ margin: "0 0 14px" }}>Food Order System</h1>

				<nav style={{ display: "flex", gap: 10, marginBottom: 20 }}>
					<NavLink to="/order" style={linkStyle}>Order Management</NavLink>
					<NavLink to="/coupons" style={linkStyle}>Promo Processing</NavLink>
				</nav>

				<Routes>
					<Route path="/order" element={<OrderPage />} />
					<Route path="/coupons" element={<CouponPage />} />
					<Route path="*" element={<Navigate to="/order" replace />} />
				</Routes>
			</div>
		</BrowserRouter>
	);
}
