import React,{useEffect,useState} from "react";
import {listProducts} from "./api";

export default function ProductList(){

const [products,setProducts] = useState([])

useEffect(()=>{

listProducts().then(setProducts)

},[])

return(

<div>

<h2>Products</h2>

{products.map(p=>(
<div key={p.id}>
{p.name} - ${p.price}
</div>
))}

</div>

)

}

