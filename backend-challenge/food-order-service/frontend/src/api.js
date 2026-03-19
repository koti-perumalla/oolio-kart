export async function uploadFile(file){

const data = new FormData()

data.append("file",file)

return fetch("/api/upload",{
method:"POST",
body:data
})

}

export async function processCoupons(){

return fetch("/api/process",{method:"POST"})

}

export async function listProducts(){

return fetch("/api/product").then(r=>r.json())

}

export async function placeOrder(payload){

return fetch("/api/order",{
method:"POST",
headers:{
"Content-Type":"application/json",
"api_key":process.env.REACT_APP_API_KEY
},
body:JSON.stringify(payload)
})

}

export async function processingStatus(){

return fetch("/api/processing-status").then(r=>r.json())

}

