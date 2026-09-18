'use client'

import { useState, useEffect } from 'react'
import { useRouter } from 'next/navigation';
import { setMemoryAccessToken, api } from '@/app/dashboard/api';

//outside random words
const words = ["PAPER", "DOOR", "THIN", "GRASS", "GRAY", "MINE", "CHALK", "CAT", "DOG", "RUN", "FAST", "BIG", "RED", "SUN", "HAT", "CUP", "PEN", "BOX", "CAR", "SKY", "SIT", "MAP", "NET", "BED", "TOY", "PIG", "PAN"];

const surrealWords = () => {
  const index = Math.floor(Math.random() * words.length);
  return words[index];
};

export default function AuthPage() {

  const router = useRouter();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [successId, setSuccessId] = useState<number | null>(null);
  const [loginMessage, setLoginMessage] = useState('');

  const [selectedWords, setSelectedWords] = useState<string[]>(["","","","","",""]);

  useEffect(() => {
    let timeoutId: NodeJS.Timeout;

    const shuffleWords = () => {
      const shuffledWords = [...words].sort(() => 0.5 - Math.random());
      setSelectedWords(shuffledWords.slice(0,6));

      const delay = Math.random() * 1000 + 500;
      timeoutId = setTimeout(shuffleWords, delay);
    };

    shuffleWords();
    return () => clearTimeout(timeoutId);

  },[]);

  useEffect(() => {

    if (successId || error || loginMessage) {
      const timer = setTimeout(() => {
        setSuccessId(null);
        setError('');
        setLoginMessage('');
      }, 2000);

      return () => clearTimeout(timer);
    }
  }, [successId, error, loginMessage]);

  const validateInput = () => {
    if (!email || !password || /\s/.test(email)) {
      setError('PLEASE FILL ALL FIELDS (No space in email)');
      return false;
    }

    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailRegex.test(email)) {
      setError('Invalid Email Format');
      return false;
    }
    if (password.length < 6) {
      setError('PASSWORD MUST BE AT LEAST 6 CHARACTERS');
      return false;
    }
    return true;
  }

  const handleSignup = async (formSubmitEvent: React.SyntheticEvent) => {
    formSubmitEvent.preventDefault();
    setSuccessId(null);
    setLoginMessage('');
    setError('');

    if(!validateInput()) return;

    setLoading(true);
    try {
      const response = await api.post(`/api/register`, { email, password });
      setSuccessId(response.data.id);
    } catch (error: any) {
      if (error.response && typeof error.response.data == 'string') {
        setError(error.response.data.trim());
      } else {
        setError("Network error. Please check your connection");
      }
    } finally {
      setLoading(false);
    }
  }

  const handleLogin = async (formSubmitEvent: React.SyntheticEvent) => {
    formSubmitEvent.preventDefault();
    setSuccessId(null);
    setLoginMessage('');
    setError('');

    if (!email || !password){
      setError('PLEASE FILL or COMPLETE')
      return;
    }

    setLoading(true);
    try {
      const response = await api.post('/api/login', { email, password });
      console.log("LOGIN RESPONSE DATA:", response.data);
      setLoginMessage('Login Successful');
      setMemoryAccessToken(response.data.access_token);
      router.push('/dashboard');
    }catch(error: any){
      if(error.response && typeof error.response.data == 'string'){
        setError(error.response.data.trim());
      } else {
        setError("Network error. Please check your connection");
      }
    }finally{
      setLoading(false);
    }
  }

  const handleViewerMode = async (formSubmitEvent: React.SyntheticEvent) => {
    formSubmitEvent.preventDefault();
    setSuccessId(null);
    setLoginMessage('');
    setError('');

    setLoading(true);
    try {
      const response = await api.post('/api/viewer');
      setMemoryAccessToken(response.data.access_token);
      router.push('/dashboard');
    } catch (error: any){
      if (error.response && typeof error.response.data == 'string') {
        setError(error.response.data.trim())
      }else{
        setError('Network error');
      }
    }finally{
      setLoading(false);
    }
  }

  const isVisible = loading || successId !== null || error !== '' || loginMessage !== '';

  return (
    <main className="min-h-screen bg-black text-white font-mono p-8 flex flex-col justify-center items-center relative">
      <div className="absolute top-20 w-full max-w-md px-4 flex flex-col items-center pointer-events-none">
        <div className={`w-full p-3 bg-zinc-950 border ${error ? 'border-red-900 text-red-400' : 'border-zinc-700 text-white'} text-xs text-center shadow-2xl transition-all duration-700 ease-out ${isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 -translate-y-2'}`}>
          
          {successId && !loading && `[ REGISTERED SUCCESSFULLY! ID: ${successId} ]`} {/**testing purposes, might remove the ID */}
          {loginMessage && !loading && `[${loginMessage}]`}
          {error && !loading && `[ ${error} ]`}
        </div>
      </div>

      <div className="border border-zinc-800 p-6 w-full max-w-md bg-zinc-950">
        <h1 className="text-sm tracking-widest text-zinc-400 mb-6 uppercase">
          {selectedWords[0]}_{selectedWords[1]} {selectedWords[2]}_{selectedWords[3]} {selectedWords[4]}_{selectedWords[5]}
        </h1>
        <div className="flex flex-col gap-4">
          <input 
            type="email" 
            placeholder="EMAIL"
            value={email}
            onChange={(changeEvent) => setEmail(changeEvent.target.value)}
            className="bg-black border border-zinc-700 p-2 text-sm focus:outline-none focus:border-white"
          />
          <input 
            type="password" 
            placeholder="PASSWORD"
            value={password}
            onChange={(changeEvent) => setPassword(changeEvent.target.value)}
            className="bg-black border border-zinc-700 p-2 text-sm focus:outline-none focus:border-white"
          />

          <div className="flex gap-2 mt-4">
            <button 
              type="button"
              onClick={handleLogin}
              className="flex-1 bg-white text-black text-xs font-bold py-2 hover:bg-zinc-200"
            >
              LOGIN
            </button>
            <button 
              type="button"
              onClick={handleSignup}
              className="flex-1 border border-zinc-700 text-xs py-2 hover:border-white"
            >
              SIGN_UP
            </button>
          </div>

          <button 
            type="button"
            onClick={handleViewerMode}
            className="mt-2 text-zinc-500 text-xs hover:text-white underline text-center"
          >
            Enter Viewer Mode
          </button>
        </div>
      </div>
    </main>
  )
}