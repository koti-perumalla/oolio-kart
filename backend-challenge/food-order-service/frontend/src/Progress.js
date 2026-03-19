import React,{useEffect,useState} from "react";
import {processingStatus} from "./api";

export default function Progress(){

const [count,setCount]=useState(0)
const [isRunning,setIsRunning]=useState(false)
const [lastCompletedAt,setLastCompletedAt]=useState("")
const [runTotalLines,setRunTotalLines]=useState(0)
const [runValid,setRunValid]=useState(0)
const [runInvalidFormat,setRunInvalidFormat]=useState(0)
const [runDuplicates,setRunDuplicates]=useState(0)
const [runPersisted,setRunPersisted]=useState(0)

useEffect(()=>{

const timer=setInterval(async()=>{

    const d=await processingStatus()

    setCount(d.totalProcessed || 0)
    setIsRunning(Boolean(d.isRunning))
    setLastCompletedAt(d.lastCompletedAt || "")
    setRunTotalLines(d.currentRunTotalLines || 0)
    setRunValid(d.currentRunValid || 0)
    setRunInvalidFormat(d.currentRunInvalidFormat || 0)
    setRunDuplicates(d.currentRunDuplicatesWithinFile || 0)
    setRunPersisted(d.currentRunPersisted || 0)

    },2000)

    return()=>clearInterval(timer)

    },[])

    return(

    <div>

    <h3>Processing Progress</h3>
    <p>Total Processed: <b>{count}</b></p>
    <p>Status: <b>{isRunning ? "Running" : "Idle"}</b></p>
    <p>Current Run Total Lines: <b>{runTotalLines}</b></p>
    <p>Current Run Valid: <b>{runValid}</b></p>
    <p>Current Run Invalid Format: <b>{runInvalidFormat}</b></p>
    <p>Current Run Duplicates (Within File): <b>{runDuplicates}</b></p>
    <p>Current Run Persisted: <b>{runPersisted}</b></p>
    {lastCompletedAt && <p>Last Completed: {new Date(lastCompletedAt).toLocaleString()}</p>}

    </div>

)

}