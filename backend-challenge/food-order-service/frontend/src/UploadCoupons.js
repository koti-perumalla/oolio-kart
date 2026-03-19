import React,{useState} from "react";
import {uploadFile,processCoupons} from "./api";

export default function UploadCoupons(){

const [files,setFiles] = useState([])
const [message,setMessage] = useState("")
const [error,setError] = useState("")

const upload = async()=>{

setMessage("")
setError("")

if(!files || files.length !== 3){
setError("Please select exactly 3 promo files.")
return
}

for(let f of files){
const res = await uploadFile(f)
if(!res.ok){
const text = await res.text()
setError(text || "Upload failed")
return
}
}

setMessage("All 3 promo files uploaded successfully.")

}

const process = async()=>{
setMessage("")
setError("")

const res = await processCoupons()

if(!res.ok){
const text = await res.text()
setError(text || "Processing start failed")
return
}

setMessage("Processing started.")

}

return(

<div>

<h3>Upload 3 Promo Files</h3>

<input type="file" multiple onChange={e=>setFiles(e.target.files)} />

<div style={{marginTop:12,display:"flex",gap:8}}>
<button onClick={upload}>Upload</button>

<button onClick={process}>Process</button>
</div>

{message && <p style={{color:"#166534",marginTop:10}}>{message}</p>}
{error && <p style={{color:"#b91c1c",marginTop:10}}>{error}</p>}

</div>

)

}
